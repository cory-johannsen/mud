# Hero Point System Design Spec

**Date:** 2026-03-14
**Status:** Design Specification
**Document ID:** HEROPOINT-2026-03-14

## Executive Summary

This specification defines a Hero Point resource system for the MUD game, enabling players to spend hero points to reroll recent ability checks (taking the higher result) or to stabilize when dying. Hero points are awarded at level-up, via GM grant command, and are persisted in the database per character.

## 1. Goals

GOAL-1: Provide players with a reroll mechanic to mitigate negative RNG outcomes.
GOAL-2: Provide an emergency mechanic to recover from death, creating dramatic recovery moments.
GOAL-3: Allow GMs to award hero points as recognition for exceptional roleplay or heroic moments.
GOAL-4: Ensure hero point state is persistent and survives session restarts.

## 2. Feature 1: Data Model

### REQ-DM1
The `PlayerSession` struct MUST add four new fields to track hero point state:
- `HeroPoints int` — current count of available hero points (persisted to database)
- `LastCheckRoll int` — dice result of the most recent ability check (session-only, default 0 means no check recorded)
- `LastCheckDC int` — DC of the most recent ability check (session-only)
- `LastCheckName string` — display name of the most recent check type (e.g., "Perception", "Grapple", "Acrobatics") for user messaging (session-only)

### REQ-DM2
The character saver interface MUST add the following method:
```go
SaveHeroPoints(ctx context.Context, characterID int64, heroPoints int) error
```

### REQ-DM3
Upon session start, `HeroPoints` MUST be loaded from the database for the character. For characters without a persisted hero point count, the default value MUST be 0.

### REQ-DM4
`LastCheckRoll`, `LastCheckDC`, and `LastCheckName` MUST NOT be persisted to the database; they MUST be stored only in memory during the session.

## 3. Feature 2: Award Events

### REQ-AWARD1
When a character levels up, the XP handler MUST award exactly 1 hero point by incrementing `HeroPoints` and calling `SaveHeroPoints` to persist the change.

### REQ-AWARD2
A new subcommand `grant heropoint <player>` MUST be added to the existing editor `grant` command. When invoked, it MUST:
- Award exactly 1 hero point to the target player's current session
- Call `SaveHeroPoints` to persist the change
- Send a notification message to the target player indicating they received a hero point from the GM
- Send a confirmation message to the granting GM

### REQ-AWARD3
Upon session start, if `HeroPoints` is not explicitly set, the session MUST load the persisted hero point count via the character saver; no other award event occurs at session start.

## 4. Feature 3: Hero Point Command

### REQ-CMD1
A new command `heropoint` MUST be added to the game command system, implemented per the full CMD pipeline (commands.go handler constant, BuiltinCommands entry, dedicated handler file, proto message, bridge handler, gRPC service dispatch).

### REQ-CMD2
The `heropoint` command MUST support exactly two subcommands: `reroll` and `stabilize`, dispatched within a single handler function.

### Subcommand: heropoint reroll

### REQ-REROLL1
The `heropoint reroll` subcommand MUST validate the following preconditions before allowing execution:
- `HeroPoints >= 1` (player has at least one hero point)
- `LastCheckRoll != 0` (a recent check has been recorded in this session)

### REQ-REROLL2
If preconditions are not met, the subcommand MUST return an error and MUST NOT consume any hero point.

### REQ-REROLL3
When `heropoint reroll` executes successfully, it MUST:
1. Roll fresh 1d20
2. Determine the winner by selecting `max(LastCheckRoll, NewRoll)`
3. Update `LastCheckRoll` to the winner value
4. Decrement `HeroPoints` by exactly 1
5. Call `SaveHeroPoints` to persist the change

### REQ-REROLL4
Upon successful execution of `heropoint reroll`, the player MUST receive the message:
```
"You spend a hero point. Original roll: {LastCheckRoll}, New roll: {NewRoll} — keeping {Winner}."
```

### Subcommand: heropoint stabilize

### REQ-STABILIZE1
The `heropoint stabilize` subcommand MUST validate the following preconditions before allowing execution:
- `HeroPoints >= 1` (player has at least one hero point)
- `sess.Dead == true` (character is currently dying)

### REQ-STABILIZE2
If preconditions are not met, the subcommand MUST return an error and MUST NOT consume any hero point.

### REQ-STABILIZE3
When `heropoint stabilize` executes successfully, it MUST:
1. Set `sess.CurrentHP` to 0
2. Set `sess.Dead` to false
3. Decrement `HeroPoints` by exactly 1
4. Call `SaveHeroPoints` to persist the change

### REQ-STABILIZE4
Upon successful execution of `heropoint stabilize`, the player MUST receive the message:
```
"You spend a hero point, pulling back from the brink. You stabilize at 0 HP."
```

## 5. Feature 4: Display and Documentation

### REQ-DISPLAY1
The character sheet display MUST include a new line showing the current hero point count:
```
Hero Points: N
```
This line MUST appear near the top of the character sheet stats.

### REQ-DISPLAY2
The FEATURES.md file MUST be updated to mark hero point-related items as in-scope with status indicators (e.g., `[x]` for complete, `[ ]` for future). Specifically, the Bosses section MUST include the item:
```
[ ] Award 1 hero point on boss kill
```

## 6. Testing Requirements

### REQ-T1
The character saver MUST support round-trip save and load of `HeroPoints`. Test MUST verify that `SaveHeroPoints` followed by loading a fresh session yields the same hero point count.

### REQ-T2
`heropoint reroll` with no recorded check (`LastCheckRoll == 0`) MUST return an error and MUST NOT decrement `HeroPoints`.

### REQ-T3
`heropoint reroll` with 0 hero points MUST return an error and MUST NOT process the reroll.

### REQ-T4
`heropoint reroll` with `LastCheckRoll=8` and fresh roll=15 MUST result in `LastCheckRoll=15`, `HeroPoints` decremented by 1, and the player receiving the appropriate message indicating the winner is 15.

### REQ-T5
`heropoint reroll` with `LastCheckRoll=15` and fresh roll=8 MUST result in `LastCheckRoll=15` (original is kept), `HeroPoints` decremented by 1, and the player receiving the appropriate message indicating the winner is 15.

### REQ-T6
`heropoint stabilize` when the character is not dying (`sess.Dead == false`) MUST return an error and MUST NOT decrement `HeroPoints`.

### REQ-T7
`heropoint stabilize` when the character is dying MUST set `sess.Dead` to false, `sess.CurrentHP` to 0, decrement `HeroPoints` by 1, and persist the change.

### REQ-T8
When a character levels up, the XP handler MUST award exactly 1 hero point and persist it via `SaveHeroPoints`.

### REQ-T9
The `grant heropoint <player>` subcommand MUST award exactly 1 hero point to the target player's session, persist the change, send a notification to the target, and send a confirmation to the granting GM.

### REQ-T10
Property-based test: for any `HeroPoints` in [0, 10] and `LastCheckRoll` in [1, 20], after a successful reroll the following invariants MUST hold:
- Total `HeroPoints` decrements by exactly 1
- Winning roll is >= original `LastCheckRoll`
- No error is returned

## 7. Implementation Order

1. **Data Model:** Add fields to `PlayerSession`, implement `SaveHeroPoints` in character saver
2. **Database Persistence:** Ensure load/save round-trip works via unit tests
3. **Award Events:** Implement level-up award and `grant heropoint` subcommand
4. **Hero Point Command:** Implement full CMD pipeline (reroll and stabilize subcommands)
5. **Display:** Update character sheet rendering to show hero point count
6. **Documentation:** Update FEATURES.md with hero point tracking items
7. **Testing:** Ensure all REQ-T items pass before marking feature complete

## 8. Acceptance Criteria

ACCEPT-1: All REQ-* items MUST be satisfied.
ACCEPT-2: All REQ-T* tests MUST pass.
ACCEPT-3: The full CMD pipeline for `heropoint` MUST be wired end-to-end (handler constant, BuiltinCommands entry, dedicated handler file, proto message, bridge handler, gRPC dispatch).
ACCEPT-4: Character sheet MUST display hero point count.
ACCEPT-5: FEATURES.md MUST be updated with hero point tracking.
ACCEPT-6: No TODOs, placeholders, or incomplete code MUST be present in the implementation.

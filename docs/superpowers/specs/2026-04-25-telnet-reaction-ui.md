---
title: Telnet Reaction UI — Prompt, Ready Command, R Budget Badge
issue: https://github.com/cory-johannsen/mud/issues/268
date: 2026-04-25
status: spec
prefix: TRXN
depends_on:
  - "#244 Reactions and Ready actions (REACTION-* — proto pair, web modal, server-side hub)"
related: []
---

# Telnet Reaction UI

## 1. Summary

The reaction system from #244 is mostly wired:

- Proto: `ServerEvent.ReactionPromptEvent` (field 43) and `ClientMessage.ReactionResponse` (field 141) are defined (`api/proto/game/v1/game.proto:374,177,382-398`).
- Server: `ReactionPromptHub`, `fireTrigger`, `buildReactionCallback`, the Ready action's `ActionReady`, and the reaction budget all exist and have tests.
- Web client: `ReactionPromptModal.tsx` reads `state.reactionPrompt`, renders option buttons with countdown, and sends `ReactionResponse` on click.
- Server: the `ready` bridge handler exists at `internal/frontend/handlers/bridge_handlers.go:1263-1281` and queues an `ActionReady` per player input.

Three telnet-side pieces are still dark:

1. **No event routing**: `forwardServerEvents` in `internal/frontend/handlers/game_bridge.go` has no case for `*gamev1.ServerEvent_ReactionPromptEvent` — the prompt is dispatched server-side but never reaches the telnet client.
2. **No connection state for reactions**: `conn.go` has `TabCompleteResponse` as a precedent for buffered server-event channels, but no `ReactionInputCh` or `reactionBuf`. The telnet client cannot capture `y/n/1..N` and route it to the right `ReactionResponse`.
3. **No on-screen surface**: `WritePromptSplit` (`internal/frontend/telnet/screen.go:459-479`) does not render the reaction prompt or the live `R:N` budget badge. The player has no visual indication that a reaction is required.

This spec adds the three missing pieces. The protocol is unchanged; this is purely the telnet client integration.

## 2. Goals & Non-Goals

### 2.1 Goals

- TRXN-G1: Telnet players in combat see a one-line reaction prompt with a live countdown and the option list.
- TRXN-G2: `y / n / 1..N` typed during the prompt window dispatches a `ReactionResponse` with the matching option id (or empty string for `n` / skip).
- TRXN-G3: Other commands typed during the prompt window are buffered and replayed once the prompt closes (or the player explicitly accepts a reaction).
- TRXN-G4: Chat commands `say` and `'` (the apostrophe alias) bypass the buffer and execute immediately, so players can talk while deciding.
- TRXN-G5: The prompt row shows an `R:N` badge where N is the player's remaining reaction budget for the current round.
- TRXN-G6: A `ready <action> when <trigger>` command queues `ActionReady` (the bridge handler already exists; this requires only its inclusion in the user-facing help / help text).

### 2.2 Non-Goals

- TRXN-NG1: Changes to `fireTrigger`, `ReactionPromptHub`, or `buildReactionCallback`. Already shipped via #244.
- TRXN-NG2: Web client work. Already shipped via #244 Task 13.
- TRXN-NG3: New reaction types. The trigger / response model from #244 is the contract.
- TRXN-NG4: ANSI animations or color schemes — countdown is plain text.
- TRXN-NG5: Per-character preferences for default reaction (e.g., "always Y to Reactive Strike"). Future ticket.
- TRXN-NG6: Auto-decline on disconnect — `ReactionPromptHub` handles its own timeout server-side.

## 3. Glossary

- **Reaction prompt**: a `ReactionPromptEvent` from the server asking the client to pick one of `repeated options` within `deadline_unix_ms`.
- **Buffered command**: a non-reaction command typed during the prompt window, held in the connection's `reactionBuf` until the prompt closes.
- **R-budget**: the player's remaining reactions this round (per Combatant.ReactionBudget from #244).
- **Reaction option id**: the `option.id` field on `ReactionPromptEvent.options[]`. Empty string in `ReactionResponse.chosen` means "skip".

## 4. Requirements

### 4.1 Connection State

- TRXN-1: `internal/frontend/telnet/conn.go` `Conn` struct MUST gain three fields:
  - `ReactionPrompt *gamev1.ReactionPromptEvent` — non-nil while a prompt is active.
  - `ReactionBuf []string` — buffered raw input lines awaiting prompt closure.
  - `ReactionInputCh chan string` — a 1-buffered channel for routing the player's chosen option (`y` / `n` / digit / `id-string`) to the dispatcher goroutine.
- TRXN-2: `Conn.Lock()` MUST guard reads/writes of `ReactionPrompt` and `ReactionBuf`. The channel is goroutine-safe by construction.
- TRXN-3: When a prompt is active, the prompt's deadline countdown MUST be re-rendered every second via the existing display tick.

### 4.2 Server Event Routing

- TRXN-4: `forwardServerEvents` in `internal/frontend/handlers/game_bridge.go` MUST gain a case for `*gamev1.ServerEvent_ReactionPromptEvent`:
  - Acquire `Conn.Lock()`.
  - Set `Conn.ReactionPrompt` to the event payload.
  - Render the prompt to the prompt row (TRXN-9).
  - Start a goroutine that:
    - Listens on `Conn.ReactionInputCh` for the player's response.
    - Listens on a `time.After(deadline - now)` timer.
    - On either, builds a `ClientMessage_ReactionResponse{ prompt_id, chosen }` and sends to the gameserver.
    - Clears `Conn.ReactionPrompt`, drains `Conn.ReactionBuf` back into the input pipeline, releases the lock.
- TRXN-5: When a `ReactionPromptEvent` arrives while another prompt is already active (rare; possible if server fires two reactions back-to-back faster than the client can resolve), the new prompt MUST replace the old one. The pending prompt's goroutine receives a cancel signal and MUST send a skip response (`chosen: ""`) for the obsoleted prompt.
- TRXN-6: The cancel mechanism MUST use a per-prompt `context.Context` derived from the connection context.

### 4.3 Input Dispatch

- TRXN-7: The telnet command dispatcher (the function that reads a line from the player and routes it to the bridge) MUST consult `Conn.ReactionPrompt` first:
  - If non-nil and the line matches one of: `y`, `n`, a digit `1..N` matching an option index, or the literal `option.id` of any option:
    - Translate to the option id (or empty for `n` / skip).
    - Send on `Conn.ReactionInputCh`.
    - Return without further routing.
  - If non-nil and the line starts with `say ` or `'`:
    - Bypass the buffer, route normally to chat.
  - If non-nil and the line is anything else:
    - Append to `Conn.ReactionBuf`.
    - Print a single hint to the player: `(buffered until reaction resolves)`.
- TRXN-8: When the prompt closes (response sent or timeout), the dispatcher MUST replay buffered lines in order to the normal input pipeline. The buffer is drained atomically so a second prompt arriving mid-replay does not interleave.

### 4.4 On-Screen Rendering

- TRXN-9: A new helper `screen.WriteReactionPrompt(prompt *ReactionPromptEvent, secondsLeft int)` MUST be added to `internal/frontend/telnet/screen.go`. Output format on the prompt row:
  ```
  REACT (Reactive Strike)? [Y]es [N]o  1) Strike  2) Trip   <12s>
  ```
  When more than 9 options exist, the rendering wraps to the console region with a `[…]` continuation indicator. Likely uncommon — single-option prompts are the norm.
- TRXN-10: `WritePromptSplit` MUST gain a parameter or sibling helper that renders the `R:N` badge as a prefix when the player has a non-zero remaining budget. Format: `[R:1] > ` for budget 1; `[R:0] > ` when fully spent; absent when not in combat.
- TRXN-11: The reaction prompt rendering MUST NOT mutate the room view region or the console scrollback. Updates land on the prompt row only.

### 4.5 Ready Command Surfacing

- TRXN-12: The existing `ready` bridge handler at `bridge_handlers.go:1263-1281` MUST be added to the telnet `help` command's combat-actions section so players discover it.
- TRXN-13: The `ready` command's parse error messages MUST include a usage example: `usage: ready <action> when <trigger>` with the supported actions (`strike`, `step`, `shield`) and triggers (`enters`, `attacks`, `ally`).
- TRXN-14: When the player issues `ready` while not in combat, the server already rejects with a clear error; no telnet-side change required, but the telnet `help` text MUST note "(combat only)".

### 4.6 Tests

- TRXN-15: New tests in `internal/frontend/telnet/conn_reaction_test.go` MUST cover:
  - Prompt arrives → connection state set; goroutine started.
  - `y` / `n` / `1` / `2` / option id all translate correctly.
  - Other commands buffered; replayed on prompt close.
  - `say hello` and `' hello` bypass buffer and dispatch immediately.
  - Timeout fires → skip response sent; buffer replayed.
  - Concurrent prompt arrival cancels the prior; obsoleted prompt skipped.
- TRXN-16: A new test in `internal/frontend/handlers/game_bridge_test.go` MUST verify the routing case for `ServerEvent_ReactionPromptEvent` (TRXN-4).
- TRXN-17: An integration test under `internal/gameserver/grpc_service_telnet_reaction_test.go` (or equivalent) MUST exercise the full path: server fires a Reactive Strike trigger, telnet player sees prompt, replies `y`, server processes the response. This requires a telnet-aware test harness; the implementer SHOULD reuse the existing tab-complete integration test as a template.

## 5. Architecture

### 5.1 Where the new code lives

```
internal/frontend/telnet/
  conn.go                              # ReactionPrompt, ReactionBuf, ReactionInputCh fields
  screen.go                            # WriteReactionPrompt + R:N badge in WritePromptSplit
  conn_reaction_test.go                # NEW

internal/frontend/handlers/
  game_bridge.go                       # forwardServerEvents → handle ReactionPromptEvent
  game_bridge_test.go                  # NEW: routing case test
  bridge_handlers.go                   # `help` text addition for `ready`

# No proto changes — protocol already exists from #244.
```

### 5.2 Prompt lifecycle

```
server fires a trigger → ReactionPromptHub builds prompt → forwardServerEvents
   │
   ▼
case ServerEvent_ReactionPromptEvent:
   conn.Lock(); conn.ReactionPrompt = ev; conn.Unlock()
   screen.WriteReactionPrompt(ev, secondsLeft=ev.deadline - now)
   start goroutine:
       select {
         case opt := <-conn.ReactionInputCh:  // player input
         case <-time.After(deadline-now):     // timeout
       }
       send ClientMessage_ReactionResponse{ prompt_id: ev.id, chosen: opt }
       conn.Lock(); conn.ReactionPrompt = nil; replay conn.ReactionBuf; conn.Unlock()

input dispatcher:
   line := readLine()
   if conn.ReactionPrompt != nil:
       if y/n/digit/id matches: conn.ReactionInputCh <- translatedOpt; return
       if starts with say/': route to chat normally
       else: conn.ReactionBuf = append(conn.ReactionBuf, line); print hint
   else:
       route normally
```

### 5.3 Single sources of truth

- Reaction prompt protocol: `api/proto/game/v1/game.proto` (already shipped).
- Telnet connection reaction state: `Conn` struct only.
- Prompt rendering: `screen.WriteReactionPrompt` only.
- Input dispatch logic: the telnet input goroutine — single function.

## 6. Open Questions

- TRXN-Q1: Does the buffer have a size limit? Recommendation: cap at 64 lines. Beyond that, drop oldest with a "(buffer full, oldest dropped)" hint.
- TRXN-Q2: When the player types a partial line and the prompt arrives, does the partial line buffer or dispatch on the next newline? Recommendation: Dispatch on next newline — the keystroke buffer is unaffected; only completed lines are buffered. This matches the existing dispatcher semantics.
- TRXN-Q3: When the prompt has many options (>9), do we use letters `a–z` after `1–9`? Recommendation: yes. Cap at 26.
- TRXN-Q4: The `R:N` badge — should it surface in non-combat too (where N is undefined / 0)? Recommendation: hide outside combat; it is a combat-mode element only.
- TRXN-Q5: The hint `(buffered until reaction resolves)` could be noisy for players who type rapidly during prompts. Recommendation: print only once per prompt, even if multiple lines are buffered.

## 7. Acceptance

- [ ] Telnet player in combat sees the reaction prompt on the prompt row with a live countdown.
- [ ] `y` / `n` / `1..N` / option id during the prompt sends `ReactionResponse` correctly.
- [ ] Other commands typed during the prompt are buffered and replayed once the prompt closes.
- [ ] `say` and `'` bypass the buffer and execute immediately.
- [ ] Prompt row shows `R:N` badge where N matches the server-side `Combatant.ReactionBudget`.
- [ ] `ready strike when attacks` queues an `ActionReady` correctly; `help` lists the syntax.
- [ ] Timeout fires a skip response and replays the buffer.
- [ ] Concurrent prompts cancel the prior prompt with a skip; the new prompt is shown.

## 8. Out-of-Scope Follow-Ons

- TRXN-F1: Per-character default reaction preferences (auto-yes / auto-no for specific triggers).
- TRXN-F2: ANSI color / animation on the countdown.
- TRXN-F3: Multi-prompt batching (server fires three reactions, client consolidates).
- TRXN-F4: A CLI `reactions` command listing the player's available reactions before any trigger fires.

## 9. References

- Issue: https://github.com/cory-johannsen/mud/issues/268
- Predecessor spec: `docs/superpowers/specs/2026-04-21-reactions-and-ready-actions.md`
- Predecessor plan: `docs/superpowers/plans/2026-04-21-reactions-and-ready-actions.md` (Tasks 10/11/12 — original closure-based approach, replaced by gRPC pair in #244 Task 13)
- Proto pair: `api/proto/game/v1/game.proto:374` (ServerEvent field 43), `:177` (ClientMessage field 141), `:382-398` (message bodies)
- Web modal precedent: `cmd/webclient/ui/src/game/ReactionPromptModal.tsx`
- Telnet conn structure: `internal/frontend/telnet/conn.go:41-83`
- TabComplete precedent (similar buffered-channel pattern): `internal/frontend/telnet/conn.go:82` (`TabCompleteResponse`)
- Current event router: `internal/frontend/handlers/game_bridge.go:706-1154` (`forwardServerEvents`)
- Existing ready handler: `internal/frontend/handlers/bridge_handlers.go:1263-1281`
- Prompt row helper: `internal/frontend/telnet/screen.go:459-479` (`WritePromptSplit`)

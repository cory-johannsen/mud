# Currency Refactor — Crypto

## Summary

Replace the current ammunition-themed currency system (Rounds / Clips / Crates) with a single flat unit called **Crypto**. The underlying integer storage is unchanged; only the display layer, naming, and tiered decomposition logic are affected.

## Requirements

- REQ-CR-1: The currency unit MUST be renamed to `Crypto` everywhere it is displayed to the player (console messages, character sheet, inventory, NPC interactions, web UI).
- REQ-CR-2: The tiered denomination system (Rounds, Clips at 25, Crates at 500) MUST be removed. Currency MUST be displayed as a flat integer with the label `Crypto` (e.g. `512 Crypto`).
- REQ-CR-3: The constants `RoundsPerClip` and `RoundsPerCrate` in `internal/game/inventory/currency.go` MUST be removed. `DecomposeRounds` and `FormatRounds` MUST be replaced by a single `FormatCrypto(total int) string` function that returns `"{n} Crypto"`.
- REQ-CR-4: All Go identifiers that embed `Rounds`, `rounds`, `Clips`, `clips`, `Crates`, or `crates` in a currency context MUST be renamed to their `Crypto`/`crypto` equivalent (e.g. `total_rounds` → `total_crypto`, `SaveCurrency` argument names, local variable names).
- REQ-CR-5: The proto fields `InventoryView.total_rounds` (field 7) MUST be renamed to `total_crypto`. All other currency-bearing proto string fields (`CharacterSheetView.currency`, `InventoryView.currency`, `HealerView.player_currency`, `TrainerView.player_currency`) MUST continue to carry formatted Crypto strings per REQ-CR-2.
- REQ-CR-6: The grant system `GrantRequest` with `grant_type: "money"` MUST continue to function; the amount granted is interpreted as Crypto units.
- REQ-CR-7: The banker stash system MUST continue to function with the same exchange-rate mechanic, but all user-facing messages and field names MUST use `Crypto` instead of Rounds/credits.
- REQ-CR-8: All NPC cost config fields that are expressed in Rounds (healer `PricePerHP`, guard `BaseCosts`, trainer `TrainingCost`, hireling `DailyCost`, merchant `Budget`, item `base_price`) MUST continue to use the same integer values; only display labels change.
- REQ-CR-9: All console messages, web UI labels, and character sheet text that reference "rounds", "clips", or "crates" in a currency context MUST be updated to "Crypto".
- REQ-CR-10: All changes MUST be covered by updated unit tests. Any test asserting on currency format strings MUST be updated to expect `Crypto` format.

## Scope

- `internal/game/inventory/currency.go` — remove `RoundsPerClip`, `RoundsPerCrate`, `DecomposeRounds`, `FormatRounds`; add `FormatCrypto`
- `api/proto/game/v1/game.proto` — rename `InventoryView.total_rounds` → `total_crypto`; regenerate proto
- `internal/gameserver/grpc_service.go` — update all currency format calls and message strings
- `internal/game/npc/banker.go` — update stash message strings and field labels
- `internal/game/npc/merchant.go` — update any currency-referencing message strings
- `internal/game/npc/noncombat.go` — update any currency-referencing message strings
- `internal/game/session/manager.go` — rename currency-related local variables if any use rounds terminology
- `cmd/webclient/ui/src/` — update any web UI labels rendering currency (character sheet, inventory, shop)
- `internal/game/inventory/currency_test.go` (or equivalent) — update/add tests for `FormatCrypto`

## Plan

All exact line numbers and identifiers confirmed by reading source files before planning.

### Step 1 — Rewrite `internal/game/inventory/currency.go`

Replace the entire file contents. Remove `RoundsPerClip`, `RoundsPerCrate`, `DecomposeRounds`, `FormatRounds`, and the `plural` helper. Add:

```go
// FormatCrypto returns a human-readable currency string for the given total.
//
// Precondition: total >= 0.
// Postcondition: returned string is "{total} Crypto".
func FormatCrypto(total int) string {
    return fmt.Sprintf("%d Crypto", total)
}
```

Remove the `strings` import (no longer needed).

### Step 2 — Rewrite `internal/game/inventory/currency_test.go`

Replace all 8 existing test functions (which assert on Rounds/Clips/Crates strings and `DecomposeRounds`) with tests for `FormatCrypto`:

- `TestCurrency_FormatCrypto_Zero` — `FormatCrypto(0)` → `"0 Crypto"`
- `TestCurrency_FormatCrypto_One` — `FormatCrypto(1)` → `"1 Crypto"`
- `TestCurrency_FormatCrypto_Large` — `FormatCrypto(1042)` → `"1042 Crypto"`
- `TestProperty_FormatCrypto_AlwaysContainsCrypto` — property test: for any non-negative int, `FormatCrypto(n)` contains `"Crypto"` and the decimal representation of `n`

Remove `rapid` import if the new property test no longer requires it; keep if reused.

### Step 3 — Update `api/proto/game/v1/game.proto`

In the `InventoryView` message (field 7), rename:
```proto
int32 total_rounds = 7;  →  int32 total_crypto = 7;
```

Regenerate Go bindings:
```
cd api/proto/game/v1 && buf generate   # or: protoc with existing Makefile target
```

Confirm `gamev1.InventoryView` now has `TotalCrypto int32` (not `TotalRounds`).

### Step 4 — Update `internal/gameserver/grpc_service.go` (3 sites)

**Line 5338:** `Currency: inventory.FormatRounds(sess.Currency),` → `Currency: inventory.FormatCrypto(sess.Currency),`

**Line 5339:** `TotalRounds: int32(sess.Currency),` → `TotalCrypto: int32(sess.Currency),`

**Line 5353:** `return messageEvent(fmt.Sprintf("Currency: %s", inventory.FormatRounds(sess.Currency))), nil`
→ `return messageEvent(fmt.Sprintf("Currency: %s", inventory.FormatCrypto(sess.Currency))), nil`

**Line 5507:** `Currency: inventory.FormatRounds(sess.Currency),` → `Currency: inventory.FormatCrypto(sess.Currency),`

### Step 5 — Update `internal/game/session/manager.go`

**Line 53 comment:** `// Currency is the player's total rounds (ammunition-as-currency).`
→ `// Currency is the player's total Crypto.`

No identifier renames needed — `Currency`, `GetCurrency`, `AddCurrency` are already generic.

### Step 6 — Update `cmd/webclient/ui/src/proto/index.ts`

**Line 273:** `totalRounds?: number;` → `totalCrypto?: number;`

Audit all references to `totalRounds` in the web client and rename to `totalCrypto`. (Investigation found no additional usages beyond the proto interface definition.)

### Step 7 — Update web UI display components (3 files)

**`cmd/webclient/ui/src/game/drawers/InventoryDrawer.tsx` line 218:**
`{inv.currency ?? 0} credits` → `{inv.currency ?? '0 Crypto'}`

The `currency` field is already a server-formatted string (e.g. `"512 Crypto"` after Step 4); remove the hardcoded `credits` suffix.

**`cmd/webclient/ui/src/game/NpcInteractModal.tsx`:**
- Line 49: `{playerCurrency}¢` → `{playerCurrency} Crypto`
- Line 73: `You need {fullHealCost}¢ but only have {playerCurrency}¢.` → `You need {fullHealCost} Crypto but only have {playerCurrency} Crypto.`
- Line 144: `{playerCurrency}¢` → `{playerCurrency} Crypto`

Note: `playerCurrency` at these sites is a raw `int32` from the proto `player_currency` field; the "Crypto" label is appended in the component.

**`cmd/webclient/ui/src/game/NpcModal.tsx` line 164:** No change needed — `{currency}` renders the server-formatted string directly.

### Step 8 — Run test suite and commit

```
mise exec -- go test ./internal/game/inventory/... ./internal/gameserver/... ./internal/game/session/...
```

All tests must pass (100% per SWENG-6). Then commit all changes.

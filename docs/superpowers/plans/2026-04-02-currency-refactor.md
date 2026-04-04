# Currency Refactor — Crypto Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the Rounds/Clips/Crates tiered currency system with a single flat "Crypto" unit across Go backend, proto, and TypeScript web UI.

**Architecture:** The integer storage is unchanged throughout — only the display layer changes. `FormatRounds` is replaced by `FormatCrypto(total int) string` returning `"{n} Crypto"`. The proto field `InventoryView.total_rounds` is renamed `total_crypto` (same field number 7, no DB change). All display labels across telnet, web UI, and NPC modals are updated to "Crypto".

**Tech Stack:** Go, Protocol Buffers 3, TypeScript/React, `make proto` for regeneration

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `internal/game/inventory/currency.go` | Remove DecomposeRounds/FormatRounds/constants; add FormatCrypto |
| Modify | `internal/game/inventory/currency_test.go` | Replace 8 old tests with 4 FormatCrypto tests |
| Modify | `internal/game/command/char.go` | Line 58: FormatRounds → FormatCrypto |
| Modify | `internal/game/command/char_test.go` | Line 92: update assertion from "Round" to "Crypto" |
| Modify | `api/proto/game/v1/game.proto` | Rename total_rounds → total_crypto (field 7) |
| Modify (generated) | `internal/gameserver/gamev1/game.pb.go` | Regenerated via `make proto` |
| Modify | `internal/gameserver/grpc_service.go` | 4 sites: FormatRounds→FormatCrypto, TotalRounds→TotalCrypto |
| Modify | `cmd/webclient/ui/src/proto/index.ts` | totalRounds → totalCrypto |
| Modify | `cmd/webclient/ui/src/game/drawers/InventoryDrawer.tsx` | Remove " credits" suffix (currency field is already formatted) |
| Modify | `cmd/webclient/ui/src/game/NpcInteractModal.tsx` | Replace "credits"/¢ labels with "Crypto" |

---

### Task 1: Rewrite currency.go and currency_test.go

**Files:**
- Modify: `internal/game/inventory/currency.go`
- Modify: `internal/game/inventory/currency_test.go`

TDD: write the failing tests first, then rewrite the implementation.

- [ ] **Step 1: Write failing tests in `internal/game/inventory/currency_test.go`**

Replace the entire file with:

```go
package inventory

import (
	"strings"
	"testing"

	"pgregory.net/rapid"
)

func TestCurrency_FormatCrypto_Zero(t *testing.T) {
	got := FormatCrypto(0)
	if got != "0 Crypto" {
		t.Fatalf("expected %q got %q", "0 Crypto", got)
	}
}

func TestCurrency_FormatCrypto_One(t *testing.T) {
	got := FormatCrypto(1)
	if got != "1 Crypto" {
		t.Fatalf("expected %q got %q", "1 Crypto", got)
	}
}

func TestCurrency_FormatCrypto_Large(t *testing.T) {
	got := FormatCrypto(1042)
	if got != "1042 Crypto" {
		t.Fatalf("expected %q got %q", "1042 Crypto", got)
	}
}

func TestProperty_FormatCrypto_AlwaysContainsCrypto(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(0, 1_000_000).Draw(rt, "n")
		result := FormatCrypto(n)
		if !strings.Contains(result, "Crypto") {
			rt.Fatalf("FormatCrypto(%d) = %q does not contain 'Crypto'", n, result)
		}
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/inventory/... 2>&1 | head -20
```

Expected: compile error — `FormatCrypto` undefined.

- [ ] **Step 3: Rewrite `internal/game/inventory/currency.go`**

Replace the entire file with:

```go
package inventory

import "fmt"

// FormatCrypto returns a human-readable currency string for the given total.
//
// Precondition: total >= 0.
// Postcondition: returned string is "{total} Crypto".
func FormatCrypto(total int) string {
	return fmt.Sprintf("%d Crypto", total)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/inventory/... -v
```

Expected: 4 tests PASS (`TestCurrency_FormatCrypto_Zero`, `_One`, `_Large`, `TestProperty_FormatCrypto_AlwaysContainsCrypto`).

- [ ] **Step 5: Commit**

```bash
git add internal/game/inventory/currency.go internal/game/inventory/currency_test.go
git commit -m "feat(currency): replace FormatRounds with FormatCrypto; remove tiered denomination logic"
```

---

### Task 2: Update char.go and char_test.go

**Files:**
- Modify: `internal/game/command/char.go` (line 58)
- Modify: `internal/game/command/char_test.go` (line 92)

`char.go` currently calls `inventory.FormatRounds` on line 58 to render the currency section of the character sheet. The test on line 92 asserts `Contains(result, "Round")`.

- [ ] **Step 1: Update `internal/game/command/char.go` line 58**

Find:
```go
	sb.WriteString(fmt.Sprintf("%s\n", inventory.FormatRounds(sess.Currency)))
```

Replace with:
```go
	sb.WriteString(fmt.Sprintf("%s\n", inventory.FormatCrypto(sess.Currency)))
```

- [ ] **Step 2: Update `internal/game/command/char_test.go` line 92**

Find:
```go
	assert.Contains(t, result, "Round")
```

Replace with:
```go
	assert.Contains(t, result, "Crypto")
```

The comment on line 90 should also be updated. Find:
```go
	// Currency=100 rounds: 4 Clips, 0 Rounds — FormatRounds(100) = "4 Clips, 0 Rounds"
```
Replace with:
```go
	// Currency=100 crypto — FormatCrypto(100) = "100 Crypto"
```

- [ ] **Step 3: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/command/... -v -run TestHandleChar
```

Expected: `TestHandleChar_ShowsCurrency` and all other char tests PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/game/command/char.go internal/game/command/char_test.go
git commit -m "feat(currency): update char command to use FormatCrypto"
```

---

### Task 3: Update proto and regenerate Go bindings

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Modify (generated): `internal/gameserver/gamev1/game.pb.go`

`InventoryView` currently has field 7 named `total_rounds`. Rename it to `total_crypto` (keeping field number 7 — wire-compatible).

- [ ] **Step 1: Edit `api/proto/game/v1/game.proto`**

Find the `InventoryView` message. Locate:
```protobuf
    int32 total_rounds = 7;
```

Replace with:
```protobuf
    int32 total_crypto = 7;
```

- [ ] **Step 2: Regenerate Go bindings**

```bash
cd /home/cjohannsen/src/mud && make proto
```

Expected: no errors. `internal/gameserver/gamev1/game.pb.go` is updated.

- [ ] **Step 3: Verify compilation fails (as expected — grpc_service.go still uses TotalRounds)**

```bash
cd /home/cjohannsen/src/mud && go build ./... 2>&1 | grep -i "totalrounds\|TotalRounds"
```

Expected: compile errors referencing `TotalRounds` in `grpc_service.go`. This confirms the field rename propagated correctly and Task 4 is needed.

- [ ] **Step 4: Commit proto only**

```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/game.pb.go
git commit -m "feat(proto): rename InventoryView.total_rounds to total_crypto (field 7)"
```

---

### Task 4: Update grpc_service.go

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (4 sites)

After the proto rename, `TotalRounds` is gone from `gamev1.InventoryView`. Four call sites in `grpc_service.go` need updating.

- [ ] **Step 1: Fix line 5338 — FormatRounds call in InventoryView builder**

Find:
```go
		Currency:    inventory.FormatRounds(sess.Currency),
```
Replace with:
```go
		Currency:    inventory.FormatCrypto(sess.Currency),
```

- [ ] **Step 2: Fix line 5339 — TotalRounds field assignment**

Find:
```go
		TotalRounds: int32(sess.Currency),
```
Replace with:
```go
		TotalCrypto: int32(sess.Currency),
```

- [ ] **Step 3: Fix line 5353 — FormatRounds in balance message event**

Find:
```go
	return messageEvent(fmt.Sprintf("Currency: %s", inventory.FormatRounds(sess.Currency))), nil
```
Replace with:
```go
	return messageEvent(fmt.Sprintf("Currency: %s", inventory.FormatCrypto(sess.Currency))), nil
```

- [ ] **Step 4: Fix line 5507 — FormatRounds in second InventoryView builder**

Find:
```go
		Currency:       inventory.FormatRounds(sess.Currency),
```
Replace with:
```go
		Currency:       inventory.FormatCrypto(sess.Currency),
```

- [ ] **Step 5: Update comment in `internal/game/session/manager.go` line 53**

Find:
```go
	// Currency is the player's total rounds (ammunition-as-currency).
```
Replace with:
```go
	// Currency is the player's total Crypto.
```

- [ ] **Step 6: Verify compilation**

```bash
cd /home/cjohannsen/src/mud && go build ./...
```

Expected: no errors.

- [ ] **Step 7: Run Go tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... ./internal/game/inventory/... ./internal/game/command/...
```

Expected: all tests pass.

- [ ] **Step 8: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/game/session/manager.go
git commit -m "feat(currency): update grpc_service to use FormatCrypto and TotalCrypto"
```

---

### Task 5: Update TypeScript web client

**Files:**
- Modify: `cmd/webclient/ui/src/proto/index.ts` (line 273)
- Modify: `cmd/webclient/ui/src/game/drawers/InventoryDrawer.tsx` (line 218)
- Modify: `cmd/webclient/ui/src/game/NpcInteractModal.tsx` (multiple lines)

- [ ] **Step 1: Update `cmd/webclient/ui/src/proto/index.ts`**

Find (line 273):
```typescript
  totalRounds?: number
```
Replace with:
```typescript
  totalCrypto?: number
```

- [ ] **Step 2: Update `cmd/webclient/ui/src/game/drawers/InventoryDrawer.tsx`**

Find (line 218):
```tsx
              <div style={{ marginTop: '0.25rem' }}>{inv.currency ?? 0} credits</div>
```
Replace with:
```tsx
              <div style={{ marginTop: '0.25rem' }}>{inv.currency ?? '0 Crypto'}</div>
```

Note: `inv.currency` is a server-formatted string (e.g. `"512 Crypto"`) after the Go changes. Removing the hardcoded `credits` suffix and falling back to `'0 Crypto'` when absent.

- [ ] **Step 3: Update `cmd/webclient/ui/src/game/NpcInteractModal.tsx` — HealerModal labels**

In the HealerModal section, find:
```tsx
            <div style={styles.infoRow}>
              <span style={styles.infoLabel}>Price per HP</span>
              <span style={styles.infoValue}>{pricePerHp}¢</span>
            </div>
            <div style={styles.infoRow}>
              <span style={styles.infoLabel}>Your credits</span>
              <span style={styles.infoValue}>{playerCurrency}¢</span>
            </div>
```
Replace with:
```tsx
            <div style={styles.infoRow}>
              <span style={styles.infoLabel}>Price per HP</span>
              <span style={styles.infoValue}>{pricePerHp} Crypto</span>
            </div>
            <div style={styles.infoRow}>
              <span style={styles.infoLabel}>Your Crypto</span>
              <span style={styles.infoValue}>{playerCurrency} Crypto</span>
            </div>
```

Note: the "cannot afford full heal" message also needs updating. Find:
```tsx
            <p style={styles.notice}>You need {fullHealCost}¢ but only have {playerCurrency}¢.</p>
```
Replace with:
```tsx
            <p style={styles.notice}>You need {fullHealCost} Crypto but only have {playerCurrency} Crypto.</p>
```

- [ ] **Step 4: Update `cmd/webclient/ui/src/game/NpcInteractModal.tsx` — TrainerModal**

Find (in TrainerModal):
```tsx
            <span style={styles.currency}>{playerCurrency}¢</span>
```
Replace with:
```tsx
            <span style={styles.currency}>{playerCurrency} Crypto</span>
```

- [ ] **Step 5: Update `cmd/webclient/ui/src/game/NpcInteractModal.tsx` — BankerModal**

Find (in BankerModal):
```tsx
              <span style={styles.infoLabel}>Carried credits</span>
```
Replace with:
```tsx
              <span style={styles.infoLabel}>Carried Crypto</span>
```

- [ ] **Step 6: TypeScript build**

```bash
cd /home/cjohannsen/src/mud/cmd/webclient/ui && npm run build
```

Expected: `✓ built in` — no type errors.

- [ ] **Step 7: Commit**

```bash
git add cmd/webclient/ui/src/proto/index.ts cmd/webclient/ui/src/game/drawers/InventoryDrawer.tsx cmd/webclient/ui/src/game/NpcInteractModal.tsx
git commit -m "feat(currency): update web UI to display Crypto instead of credits/rounds/¢"
```

---

### Task 6: Full test suite, mark done, deploy

**Files:**
- Modify: `docs/features/index.yaml`

- [ ] **Step 1: Run full Go test suite**

```bash
cd /home/cjohannsen/src/mud && make test
```

Expected: all packages pass.

- [ ] **Step 2: Mark feature done in index.yaml**

In `docs/features/index.yaml`, find:
```yaml
  - slug: currency-refactor
    name: Currency Refactor — Crypto
    status: planned
```
Change `status: planned` to `status: done`.

- [ ] **Step 3: Commit**

```bash
git add docs/features/index.yaml
git commit -m "feat(currency): mark currency-refactor done"
```

- [ ] **Step 4: Deploy**

```bash
cd /home/cjohannsen/src/mud && make k8s-redeploy
```

Expected: `Release "mud" has been upgraded. Happy Helming!`

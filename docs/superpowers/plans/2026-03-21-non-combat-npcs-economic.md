# Non-Combat NPCs — Economic NPCs (Merchant + Banker) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the full runtime systems and player-facing commands for Merchant and Banker non-combat NPCs, including stock/budget management, replenishment scheduling, negotiate price modifiers, stash balance, exchange rates, and all associated proto messages.

**Architecture:** All pure business logic lives in `internal/game/npc/merchant.go` and `internal/game/npc/banker.go` as package-level functions (functional core). Proto messages are added to `api/proto/game/v1/game.proto` and regenerated. Five `handle*` methods are added to `GameServiceServer` in `internal/gameserver/grpc_service_merchant.go` and three in `internal/gameserver/grpc_service_banker.go`. Session-scoped negotiate state is stored as new fields on `session.PlayerSession`. Replenishment and daily rate recalculation are wired into the existing `GameCalendar` daily-tick subscriber mechanism. Named NPC YAML files are added under `content/npcs/`.

**Tech Stack:** Go 1.26, `pgregory.net/rapid` for property-based tests, `github.com/stretchr/testify`, `gopkg.in/yaml.v3`, proto3 via `make proto`, existing `GameCalendar` subscriber pattern.

**Spec:** `docs/superpowers/specs/2026-03-20-non-combat-npcs-design.md` §2 (Merchant) and §7 (Banker).

---

## Key Existing APIs (read before implementing)

```go
// session.PlayerSession — add NegotiateModifier and NegotiatedMerchant fields (Task 1)

// npc.Manager.FindInRoom(roomID, namePrefix string) *npc.Instance
// npc.Instance.NPCType string      — "merchant" | "banker" | ...
// npc.Instance.Cowering bool       — true = non-interactive

// GameCalendar.Subscribe(fn func(GameDateTime)) — called on every game-hour tick
// GameDateTime.Hour int  (0-23) — use Hour == 0 to detect daily tick

// messageEvent(content string) *gamev1.ServerEvent  — helper in grpc_service.go
// errorEvent(content string)   *gamev1.ServerEvent

// s.sessions.GetPlayer(uid) (*session.PlayerSession, bool)
// s.npcMgr.FindInRoom(roomID, name) *npc.Instance
// s.npcMgr.InstancesInRoom(roomID) []*npc.Instance
```

---

## File Map

| File | Change |
|------|--------|
| `internal/game/session/manager.go` | Add `NegotiateModifier float64`, `NegotiatedMerchantID string`, `StashBalance int` fields to `PlayerSession` |
| `internal/game/npc/merchant.go` | New: `InitRuntimeState`, `ApplyReplenish`, `ComputeBuyPrice`, `ComputeSellPayout`, `BrowseLines`, `ApplyNegotiateOutcome` pure functions |
| `internal/game/npc/merchant_test.go` | New: unit + property-based tests for merchant logic |
| `internal/game/npc/banker.go` | New: `NewCurrentRateFromDelta`, `ComputeDeposit`, `ComputeWithdrawal` pure functions; `BankerRuntimeState` struct |
| `internal/game/npc/banker_test.go` | New: unit + property-based tests for banker logic |
| `api/proto/game/v1/game.proto` | Add `BrowseRequest`, `BuyRequest`, `SellRequest`, `NegotiateRequest`, `StashDepositRequest`, `StashWithdrawRequest`, `StashBalanceRequest` messages and oneof arms |
| `internal/gameserver/gamev1/game.pb.go` | Regenerated (do not edit by hand — run `make proto`) |
| `internal/gameserver/grpc_service_merchant.go` | New: `handleBrowse`, `handleBuy`, `handleSell`, `handleNegotiate` methods on `GameServiceServer` |
| `internal/gameserver/grpc_service_merchant_test.go` | New: unit + property-based tests for all merchant handlers |
| `internal/gameserver/grpc_service_banker.go` | New: `handleStashDeposit`, `handleStashWithdraw`, `handleStashBalance` methods on `GameServiceServer` |
| `internal/gameserver/grpc_service_banker_test.go` | New: unit + property-based tests for banker handlers |
| `internal/gameserver/grpc_service.go` | Add dispatch cases for all 7 new message types; add `merchantRuntimeStates`, `bankerRuntimeStates` map fields; wire calendar subscriber |
| `internal/gameserver/world_handler.go` | Clear negotiate modifier fields on room transition (REQ-NPC-5a) |
| `internal/game/npc/manager.go` | Add `TemplateByID`, `InstanceByID` methods; populate `templates` map in `Spawn` |
| `content/npcs/sergeant_mack.yaml` | New: weapons merchant, Last Stand Lodge |
| `content/npcs/slick_sally.yaml` | New: consumables merchant, Rusty Oasis |
| `content/npcs/whiskey_joe.yaml` | New: consumables merchant, The Bottle Shack |
| `content/npcs/old_rusty.yaml` | New: consumables merchant, The Heap |
| `content/npcs/herb.yaml` | New: consumables merchant, The Green Hell |
| `content/npcs/vera_coldcoin.yaml` | New: banker NPC, Rustbucket Ridge |

---

## Task 1: Session negotiate state and stash fields

**Files:**
- Modify: `internal/game/session/manager.go`
- Modify: `internal/game/session/manager_test.go`

- [ ] **Step 1: Write a failing test**

Add to `internal/game/session/manager_test.go`:

```go
func TestPlayerSession_NegotiateFields_DefaultZero(t *testing.T) {
    mgr := NewManager()
    _, err := mgr.AddPlayer(AddPlayerOptions{
        UID: "u1", Username: "u", CharName: "C",
        RoomID: "r1", CurrentHP: 10, MaxHP: 10, Role: "player",
    })
    require.NoError(t, err)
    sess, ok := mgr.GetPlayer("u1")
    require.True(t, ok)
    assert.Equal(t, 0.0, sess.NegotiateModifier)
    assert.Equal(t, "", sess.NegotiatedMerchantID)
    assert.Equal(t, 0, sess.StashBalance)
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
mise exec -- go test ./internal/game/session/... -run TestPlayerSession_NegotiateFields_DefaultZero -v
```

Expected: FAIL — fields do not exist.

- [ ] **Step 3: Add fields to PlayerSession**

In `internal/game/session/manager.go`, in the `PlayerSession` struct after the `WantedLevel` block, add:

```go
// NegotiateModifier is the session-scoped price modifier earned from a negotiate
// attempt with a merchant. +0.2 = 20% cheaper; -0.1 = 10% penalty on buys.
// Cleared on room transition (REQ-NPC-5a). 0.0 means no modifier active.
NegotiateModifier float64
// NegotiatedMerchantID is the instance ID of the merchant this player already
// negotiated with in the current room visit. Blocks repeat negotiate. (REQ-NPC-5)
NegotiatedMerchantID string
// StashBalance is the player's global stash credit balance, accessible at any banker.
StashBalance int
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/session/... -v 2>&1 | tail -10
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/game/session/manager.go internal/game/session/manager_test.go
git commit -m "feat: add NegotiateModifier, NegotiatedMerchantID, StashBalance to PlayerSession (REQ-NPC-5a, REQ-NPC-14)"
```

---

## Task 2: Merchant pure logic

**Files:**
- Create: `internal/game/npc/merchant.go`
- Create: `internal/game/npc/merchant_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/game/npc/merchant_test.go` (package `npc`):

```go
package npc

import (
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "pgregory.net/rapid"
)

func TestInitRuntimeState_SetsStockAndBudget(t *testing.T) {
    cfg := &MerchantConfig{
        Budget: 500,
        Inventory: []MerchantItem{
            {ItemID: "sword", BasePrice: 100, InitStock: 3, MaxStock: 5},
            {ItemID: "shield", BasePrice: 80, InitStock: 1, MaxStock: 3},
        },
        ReplenishRate: ReplenishConfig{MinHours: 1, MaxHours: 4},
    }
    state := InitRuntimeState(cfg, time.Unix(0, 0))
    assert.Equal(t, 3, state.Stock["sword"])
    assert.Equal(t, 1, state.Stock["shield"])
    assert.Equal(t, 500, state.CurrentBudget)
    assert.False(t, state.NextReplenishAt.IsZero())
}

func TestComputeBuyPrice_NoModifiers(t *testing.T) {
    got := ComputeBuyPrice(100, 1.2, 1.0, 0.0)
    assert.Equal(t, 120, got)
}

func TestComputeBuyPrice_WantedSurcharge(t *testing.T) {
    // floor(100 * 1.0 * 1.1 * (1-0.1)) = floor(99.0) = 99
    got := ComputeBuyPrice(100, 1.0, 1.1, 0.1)
    assert.Equal(t, 99, got)
}

func TestComputeBuyPrice_CritFailPenalty(t *testing.T) {
    // floor(100 * 1.0 * 1.0 * (1-(-0.1))) = floor(110) = 110
    got := ComputeBuyPrice(100, 1.0, 1.0, -0.1)
    assert.Equal(t, 110, got)
}

func TestComputeSellPayout_Basic(t *testing.T) {
    got := ComputeSellPayout(100, 0.5, 2, 0.0)
    assert.Equal(t, 100, got)
}

func TestComputeSellPayout_NegotiateBoost(t *testing.T) {
    got := ComputeSellPayout(100, 0.5, 1, 0.1)
    assert.Equal(t, 55, got)
}

func TestComputeSellPayout_CritFailUnaffected(t *testing.T) {
    // REQ-NPC-5b: crit fail does NOT penalise sells
    got := ComputeSellPayout(100, 0.5, 1, -0.1)
    assert.Equal(t, 50, got)
}

func TestApplyReplenish_StockRefill_Increments(t *testing.T) {
    cfg := &MerchantConfig{
        Budget: 500,
        Inventory: []MerchantItem{
            {ItemID: "sword", BasePrice: 100, InitStock: 0, MaxStock: 5},
        },
        ReplenishRate: ReplenishConfig{MinHours: 1, MaxHours: 1, StockRefill: 2, BudgetRefill: 100},
    }
    state := &MerchantRuntimeState{
        Stock:         map[string]int{"sword": 1},
        CurrentBudget: 200,
    }
    next := ApplyReplenish(cfg, state, 0)
    assert.Equal(t, 3, next.Stock["sword"])
    assert.Equal(t, 300, next.CurrentBudget)
}

func TestApplyReplenish_StockRefill_Zero_FullReset(t *testing.T) {
    cfg := &MerchantConfig{
        Budget: 500,
        Inventory: []MerchantItem{
            {ItemID: "sword", BasePrice: 100, InitStock: 0, MaxStock: 5},
        },
        ReplenishRate: ReplenishConfig{MinHours: 1, MaxHours: 1, StockRefill: 0, BudgetRefill: 0},
    }
    state := &MerchantRuntimeState{
        Stock:         map[string]int{"sword": 2},
        CurrentBudget: 100,
    }
    next := ApplyReplenish(cfg, state, 0)
    assert.Equal(t, 5, next.Stock["sword"])
    assert.Equal(t, 500, next.CurrentBudget)
}

func TestApplyNegotiateOutcome_CritSuccess(t *testing.T) {
    assert.Equal(t, 0.2, ApplyNegotiateOutcome("crit_success"))
}

func TestApplyNegotiateOutcome_Success(t *testing.T) {
    assert.Equal(t, 0.1, ApplyNegotiateOutcome("success"))
}

func TestApplyNegotiateOutcome_Failure(t *testing.T) {
    assert.Equal(t, 0.0, ApplyNegotiateOutcome("failure"))
}

func TestApplyNegotiateOutcome_CritFailure(t *testing.T) {
    assert.Equal(t, -0.1, ApplyNegotiateOutcome("crit_failure"))
}

func TestProperty_ComputeBuyPrice_NeverNegative(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        base := rapid.IntRange(1, 10000).Draw(rt, "base")
        margin := rapid.Float64Range(0.1, 5.0).Draw(rt, "margin")
        surcharge := rapid.Float64Range(1.0, 2.0).Draw(rt, "surcharge")
        mod := rapid.Float64Range(-0.1, 0.2).Draw(rt, "mod")
        got := ComputeBuyPrice(base, margin, surcharge, mod)
        assert.GreaterOrEqual(t, got, 0)
    })
}

func TestProperty_ComputeSellPayout_NeverNegative(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        base := rapid.IntRange(1, 10000).Draw(rt, "base")
        margin := rapid.Float64Range(0.0, 1.0).Draw(rt, "margin")
        qty := rapid.IntRange(1, 100).Draw(rt, "qty")
        mod := rapid.Float64Range(-0.1, 0.2).Draw(rt, "mod")
        got := ComputeSellPayout(base, margin, qty, mod)
        assert.GreaterOrEqual(t, got, 0)
    })
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/npc/... -run "TestInitRuntimeState|TestComputeBuy|TestComputeSell|TestApplyReplenish|TestApplyNegotiate|TestProperty_Compute" -v 2>&1 | head -20
```

Expected: FAIL — package does not compile.

- [ ] **Step 3: Create `internal/game/npc/merchant.go`**

```go
// Package npc — merchant pure-logic helpers.
package npc

import (
	"math"
	"math/rand"
	"time"
)

// InitRuntimeState creates a MerchantRuntimeState from a MerchantConfig at first
// zone initialization.
//
// Precondition: cfg must be non-nil; now must be non-zero.
// Postcondition: Stock contains one entry per Inventory item; CurrentBudget == cfg.Budget.
func InitRuntimeState(cfg *MerchantConfig, now time.Time) *MerchantRuntimeState {
	stock := make(map[string]int, len(cfg.Inventory))
	for _, item := range cfg.Inventory {
		stock[item.ItemID] = item.InitStock
	}
	return &MerchantRuntimeState{
		Stock:           stock,
		CurrentBudget:   cfg.Budget,
		NextReplenishAt: now.Add(nextReplenishInterval(cfg.ReplenishRate)),
	}
}

// nextReplenishInterval returns a random duration in [MinHours, MaxHours].
func nextReplenishInterval(r ReplenishConfig) time.Duration {
	span := r.MaxHours - r.MinHours
	h := r.MinHours
	if span > 0 {
		h += rand.Intn(span)
	}
	return time.Duration(h) * time.Hour
}

// ApplyReplenish advances a MerchantRuntimeState by one replenishment cycle.
// The unused int parameter is reserved for future danger-level-based scaling.
//
// Precondition: cfg and state must be non-nil.
// Postcondition: Returns a new MerchantRuntimeState; original is unchanged.
func ApplyReplenish(cfg *MerchantConfig, state *MerchantRuntimeState, _ int) *MerchantRuntimeState {
	next := &MerchantRuntimeState{
		Stock:           make(map[string]int, len(state.Stock)),
		CurrentBudget:   state.CurrentBudget,
		NextReplenishAt: state.NextReplenishAt.Add(nextReplenishInterval(cfg.ReplenishRate)),
	}
	for k, v := range state.Stock {
		next.Stock[k] = v
	}
	for _, item := range cfg.Inventory {
		cur := next.Stock[item.ItemID]
		if cfg.ReplenishRate.StockRefill == 0 {
			next.Stock[item.ItemID] = item.MaxStock
		} else {
			added := cur + cfg.ReplenishRate.StockRefill
			if added > item.MaxStock {
				added = item.MaxStock
			}
			next.Stock[item.ItemID] = added
		}
	}
	if cfg.ReplenishRate.BudgetRefill == 0 {
		next.CurrentBudget = cfg.Budget
	} else {
		next.CurrentBudget = state.CurrentBudget + cfg.ReplenishRate.BudgetRefill
	}
	return next
}

// ComputeBuyPrice returns the credit cost for the player to buy one unit of an
// item at basePrice, applying sellMargin, wantedSurcharge, then negotiate modifier.
//
// REQ-NPC-5b: negotiateMod is signed: +0.2 = 20% discount, -0.1 = 10% penalty.
//
// Precondition: basePrice >= 1; all multipliers > 0.
// Postcondition: Returns floor of final price; always >= 0.
func ComputeBuyPrice(basePrice int, sellMargin, wantedSurcharge, negotiateMod float64) int {
	price := float64(basePrice) * sellMargin * wantedSurcharge * (1.0 - negotiateMod)
	if price < 0 {
		price = 0
	}
	return int(math.Floor(price))
}

// ComputeSellPayout returns the credits paid to the player for selling qty units.
// Critical failure modifier (negative mod) does NOT penalise sells — clamped to 0.
//
// REQ-NPC-5b: sells are unaffected by critical failure.
// Precondition: basePrice >= 1; buyMargin in [0,1]; qty >= 1.
// Postcondition: Returns floor of payout; always >= 0.
func ComputeSellPayout(basePrice int, buyMargin float64, qty int, negotiateMod float64) int {
	mod := negotiateMod
	if mod < 0 {
		mod = 0
	}
	payout := float64(basePrice) * buyMargin * float64(qty) * (1.0 + mod)
	if payout < 0 {
		payout = 0
	}
	return int(math.Floor(payout))
}

// ApplyNegotiateOutcome converts a skill-check outcome to a session-scoped modifier.
//
// Outcomes: "crit_success"→0.2, "success"→0.1, "failure"→0.0, "crit_failure"→-0.1.
//
// Precondition: outcome is one of the four defined strings.
// Postcondition: Returns a value in {-0.1, 0.0, 0.1, 0.2}.
func ApplyNegotiateOutcome(outcome string) float64 {
	switch outcome {
	case "crit_success":
		return 0.2
	case "success":
		return 0.1
	case "crit_failure":
		return -0.1
	default:
		return 0.0
	}
}

// BrowseItem is one row in a browse listing.
type BrowseItem struct {
	ItemID    string
	BuyPrice  int
	SellPrice int
	Stock     int
}

// BrowseLines returns browse display rows for all inventory items, prices
// adjusted for the active negotiate modifier and wanted surcharge.
//
// Precondition: cfg and state must be non-nil.
// Postcondition: Returns one BrowseItem per Inventory entry in config order.
func BrowseLines(cfg *MerchantConfig, state *MerchantRuntimeState, wantedSurcharge, negotiateMod float64) []BrowseItem {
	rows := make([]BrowseItem, 0, len(cfg.Inventory))
	for _, item := range cfg.Inventory {
		rows = append(rows, BrowseItem{
			ItemID:    item.ItemID,
			BuyPrice:  ComputeBuyPrice(item.BasePrice, cfg.SellMargin, wantedSurcharge, negotiateMod),
			SellPrice: ComputeSellPayout(item.BasePrice, cfg.BuyMargin, 1, negotiateMod),
			Stock:     state.Stock[item.ItemID],
		})
	}
	return rows
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/npc/... -v 2>&1 | tail -20
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/game/npc/merchant.go internal/game/npc/merchant_test.go
git commit -m "feat: merchant pure logic — InitRuntimeState, ComputeBuyPrice, ComputeSellPayout, ApplyReplenish, BrowseLines (REQ-NPC-12, REQ-NPC-5b)"
```

---

## Task 3: Banker pure logic

**Files:**
- Create: `internal/game/npc/banker.go`
- Create: `internal/game/npc/banker_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/game/npc/banker_test.go` (package `npc`):

```go
package npc

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "pgregory.net/rapid"
)

func TestNewCurrentRate_ClampedLow(t *testing.T) {
    cfg := &BankerConfig{BaseRate: 0.5, RateVariance: 1.0}
    rate := NewCurrentRateFromDelta(cfg, -1.0)
    assert.GreaterOrEqual(t, rate, 0.5)
}

func TestNewCurrentRate_ClampedHigh(t *testing.T) {
    cfg := &BankerConfig{BaseRate: 1.0, RateVariance: 0.5}
    rate := NewCurrentRateFromDelta(cfg, 0.5)
    assert.LessOrEqual(t, rate, 1.0)
}

func TestComputeDeposit_Basic(t *testing.T) {
    assert.Equal(t, 95, ComputeDeposit(100, 0.95))
}

func TestComputeWithdrawal_Basic(t *testing.T) {
    assert.Equal(t, 100, ComputeWithdrawal(95, 0.95))
}

func TestComputeDeposit_RateOne_NoFee(t *testing.T) {
    assert.Equal(t, 200, ComputeDeposit(200, 1.0))
}

func TestComputeWithdrawal_RateOne_NoFee(t *testing.T) {
    assert.Equal(t, 200, ComputeWithdrawal(200, 1.0))
}

func TestProperty_NewCurrentRate_InRange(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        base := rapid.Float64Range(0.5, 1.0).Draw(rt, "base")
        variance := rapid.Float64Range(0.0, 0.5).Draw(rt, "variance")
        delta := rapid.Float64Range(-variance, variance).Draw(rt, "delta")
        cfg := &BankerConfig{BaseRate: base, RateVariance: variance}
        rate := NewCurrentRateFromDelta(cfg, delta)
        assert.GreaterOrEqual(t, rate, 0.5)
        assert.LessOrEqual(t, rate, 1.0)
    })
}

func TestProperty_ComputeDeposit_NeverExceedsAmount(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        amount := rapid.IntRange(1, 100000).Draw(rt, "amount")
        rate := rapid.Float64Range(0.5, 1.0).Draw(rt, "rate")
        stash := ComputeDeposit(amount, rate)
        assert.LessOrEqual(t, stash, amount)
        assert.GreaterOrEqual(t, stash, 0)
    })
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/npc/... -run "TestNewCurrentRate|TestComputeDeposit|TestComputeWithdrawal|TestProperty_New|TestProperty_ComputeDeposit" -v 2>&1 | head -15
```

Expected: FAIL.

- [ ] **Step 3: Create `internal/game/npc/banker.go`**

```go
// Package npc — banker pure-logic helpers.
package npc

import "math"

// BankerRuntimeState holds the mutable daily state for a banker NPC.
type BankerRuntimeState struct {
	// CurrentRate is the current exchange rate. Updated daily.
	// Deposit adds floor(amount * CurrentRate) to stash.
	// Withdrawal gives floor(stash / CurrentRate) carried credits.
	CurrentRate float64
}

// NewCurrentRateFromDelta computes the banker's exchange rate given a variance delta.
// Formula: clamp(BaseRate + delta, 0.5, 1.0).
//
// Precondition: cfg must be non-nil.
// Postcondition: Returns a value in [0.5, 1.0].
func NewCurrentRateFromDelta(cfg *BankerConfig, delta float64) float64 {
	rate := cfg.BaseRate + delta
	if rate < 0.5 {
		rate = 0.5
	}
	if rate > 1.0 {
		rate = 1.0
	}
	return rate
}

// ComputeDeposit returns stash credits added when a player deposits amount at rate.
// Formula: floor(amount * rate).
//
// Precondition: amount >= 0; rate in [0.5, 1.0].
// Postcondition: Returns a value in [0, amount].
func ComputeDeposit(amount int, rate float64) int {
	return int(math.Floor(float64(amount) * rate))
}

// ComputeWithdrawal returns carried credits received when withdrawing stashAmount at rate.
// Formula: floor(stashAmount / rate).
//
// Precondition: stashAmount >= 0; rate in (0, 1.0].
// Postcondition: Returns a value >= stashAmount when rate < 1.0.
func ComputeWithdrawal(stashAmount int, rate float64) int {
	if rate <= 0 {
		return stashAmount
	}
	return int(math.Floor(float64(stashAmount) / rate))
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/npc/... -v 2>&1 | tail -20
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/game/npc/banker.go internal/game/npc/banker_test.go
git commit -m "feat: banker pure logic — BankerRuntimeState, NewCurrentRateFromDelta, ComputeDeposit, ComputeWithdrawal (REQ-NPC-14)"
```

---

## Task 4: Proto messages for merchant and banker commands

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Regenerate: `internal/gameserver/gamev1/game.pb.go` (via `make proto`)

**Before starting:** Read the proto file to find the current highest field number in the `ClientMessage` oneof and the last `ReadyRequest` field number (should be 86). Confirm with:

```bash
grep -n "= [0-9]\+;" /home/cjohannsen/src/mud/api/proto/game/v1/game.proto | tail -20
```

- [ ] **Step 1: Add oneof arms to `ClientMessage`**

After the `ReadyRequest ready = 86;` arm (adjust number if different), add:

```protobuf
    BrowseRequest        browse         = 87;
    BuyRequest           buy            = 88;
    SellRequest          sell           = 89;
    NegotiateRequest     negotiate      = 90;
    StashDepositRequest  stash_deposit  = 91;
    StashWithdrawRequest stash_withdraw = 92;
    StashBalanceRequest  stash_balance  = 93;
```

- [ ] **Step 2: Add message definitions**

After the `ReadyRequest` message definition, append:

```protobuf
// BrowseRequest lists a merchant's inventory with current prices and stock.
message BrowseRequest {
  string npc_name = 1;
}

// BuyRequest purchases an item from a merchant.
message BuyRequest {
  string npc_name = 1;
  string item_id  = 2;
  int32  quantity = 3;
}

// SellRequest sells an item to a merchant.
message SellRequest {
  string npc_name = 1;
  string item_id  = 2;
  int32  quantity = 3;
}

// NegotiateRequest attempts a skill check to earn a session-scoped price modifier.
message NegotiateRequest {
  string npc_name = 1;
  // skill is "smooth_talk" or "grift". Defaults to "smooth_talk" if empty.
  string skill    = 2;
}

// StashDepositRequest deposits carried credits into the global stash.
message StashDepositRequest {
  string npc_name = 1;
  int32  amount   = 2;
}

// StashWithdrawRequest withdraws from the global stash to carried credits.
message StashWithdrawRequest {
  string npc_name = 1;
  int32  amount   = 2;
}

// StashBalanceRequest queries stash balance and current exchange rate.
message StashBalanceRequest {
  string npc_name = 1;
}
```

- [ ] **Step 3: Regenerate proto**

```bash
cd /home/cjohannsen/src/mud && make proto
```

Expected: exits 0.

- [ ] **Step 4: Verify build compiles**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./...
```

Expected: exits 0.

- [ ] **Step 5: Commit**

```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/
git commit -m "feat: add merchant and banker proto messages (browse, buy, sell, negotiate, stash deposit/withdraw/balance)"
```

---

## Task 5: NPC Manager additions (TemplateByID, InstanceByID)

**Files:**
- Modify: `internal/game/npc/manager.go`
- Modify: `internal/game/npc/manager_test.go` (or create if absent)

**Before starting:** Read `internal/game/npc/manager.go` to understand the existing Manager struct and Spawn method. Confirm whether `templates` and `instances` maps already exist.

- [ ] **Step 1: Write failing tests**

Add to `internal/game/npc/manager_test.go` (or create it):

```go
package npc

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestManager_TemplateByID_ReturnsTemplate(t *testing.T) {
    mgr := NewManager()
    tmpl := &Template{
        ID: "test_tmpl", Name: "Test", NPCType: "combat",
        MaxHP: 10, AC: 10, Level: 1,
    }
    _, err := mgr.Spawn(tmpl, "room-1")
    require.NoError(t, err)
    got := mgr.TemplateByID("test_tmpl")
    require.NotNil(t, got)
    assert.Equal(t, "test_tmpl", got.ID)
}

func TestManager_TemplateByID_MissingReturnsNil(t *testing.T) {
    mgr := NewManager()
    assert.Nil(t, mgr.TemplateByID("nonexistent"))
}

func TestManager_InstanceByID_ReturnsInstance(t *testing.T) {
    mgr := NewManager()
    tmpl := &Template{ID: "bandit", Name: "Bandit", NPCType: "combat", MaxHP: 10, AC: 10, Level: 1}
    inst, err := mgr.Spawn(tmpl, "room-1")
    require.NoError(t, err)
    got := mgr.InstanceByID(inst.ID)
    require.NotNil(t, got)
    assert.Equal(t, inst.ID, got.ID)
}

func TestManager_InstanceByID_MissingReturnsNil(t *testing.T) {
    mgr := NewManager()
    assert.Nil(t, mgr.InstanceByID("ghost"))
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/npc/... -run "TestManager_TemplateByID|TestManager_InstanceByID" -v 2>&1 | head -15
```

- [ ] **Step 3: Add TemplateByID and InstanceByID to manager.go**

Read `manager.go` first. If `templates map[string]*Template` doesn't exist, add it to Manager struct and populate in Spawn. Then add:

```go
// TemplateByID returns the Template registered under id, or nil if not found.
//
// Precondition: id must be non-empty.
// Postcondition: Returns nil if id is not registered.
func (m *Manager) TemplateByID(id string) *Template {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.templates[id]
}

// InstanceByID returns the live Instance with the given ID, or nil if not found.
//
// Precondition: id must be non-empty.
// Postcondition: Returns nil if id is not registered.
func (m *Manager) InstanceByID(id string) *Instance {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.instances[id]
}
```

- [ ] **Step 4: Run full npc tests**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/npc/... -v 2>&1 | tail -15
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/game/npc/manager.go internal/game/npc/manager_test.go
git commit -m "feat: add TemplateByID and InstanceByID to npc.Manager"
```

---

## Task 6: GameServiceServer runtime state fields

**Files:**
- Modify: `internal/gameserver/grpc_service.go`

**Before starting:** Read `internal/gameserver/grpc_service.go` to find the `GameServiceServer` struct definition and `NewGameServiceServer` constructor.

- [ ] **Step 1: Add fields to GameServiceServer struct**

In the `GameServiceServer` struct, add:

```go
// merchantRuntimeStates maps NPC instance ID to active merchant runtime state.
merchantRuntimeStates map[string]*npc.MerchantRuntimeState
// bankerRuntimeStates maps NPC instance ID to active banker runtime state.
bankerRuntimeStates map[string]*npc.BankerRuntimeState
```

- [ ] **Step 2: Initialize maps in NewGameServiceServer**

In `NewGameServiceServer`, add:

```go
s.merchantRuntimeStates = make(map[string]*npc.MerchantRuntimeState)
s.bankerRuntimeStates   = make(map[string]*npc.BankerRuntimeState)
```

- [ ] **Step 3: Verify build compiles**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go build ./... 2>&1
```

- [ ] **Step 4: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | tail -10
```

- [ ] **Step 5: Commit**

```bash
git add internal/gameserver/grpc_service.go
git commit -m "feat: add merchantRuntimeStates and bankerRuntimeStates to GameServiceServer"
```

---

## Task 7: Merchant handler methods

**Files:**
- Create: `internal/gameserver/grpc_service_merchant.go`
- Create: `internal/gameserver/grpc_service_merchant_test.go`

**Before starting:** Read `internal/gameserver/grpc_service_swim_test.go` for the `testServiceWithAdmin` helper pattern. Read one existing handler like `grpc_service_ready.go` for the handler method signature pattern. Confirm `messageEvent` helper exists in `grpc_service.go`.

- [ ] **Step 1: Write failing handler tests**

Create `internal/gameserver/grpc_service_merchant_test.go` (package `gameserver`):

```go
package gameserver

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "pgregory.net/rapid"

    "github.com/cory-johannsen/mud/internal/game/npc"
    "github.com/cory-johannsen/mud/internal/game/session"
    gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

func newMerchantTestServer(t *testing.T) (*GameServiceServer, string, *npc.Instance) {
    t.Helper()
    svc := testServiceWithAdmin(t, nil)

    uid := "merch_u1"
    _, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
        UID: uid, Username: "merch_user", CharName: "MerchChar",
        CharacterID: 1, RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
    })
    require.NoError(t, err)

    sess, _ := svc.sessions.GetPlayer(uid)
    sess.Currency = 500

    tmpl := &npc.Template{
        ID: "test_merchant", Name: "Shopkeeper", NPCType: "merchant",
        MaxHP: 20, AC: 10, Level: 1,
        Merchant: &npc.MerchantConfig{
            MerchantType: "consumables",
            SellMargin:   1.0,
            BuyMargin:    0.5,
            Budget:       300,
            Inventory: []npc.MerchantItem{
                {ItemID: "stim_pack", BasePrice: 50, InitStock: 3, MaxStock: 5},
            },
            ReplenishRate: npc.ReplenishConfig{MinHours: 1, MaxHours: 4},
        },
    }
    inst, err := svc.npcMgr.Spawn(tmpl, "room_a")
    require.NoError(t, err)
    svc.initMerchantRuntimeState(inst)

    return svc, uid, inst
}

func TestHandleBrowse_ListsInventory(t *testing.T) {
    svc, uid, inst := newMerchantTestServer(t)
    evt, err := svc.handleBrowse(uid, &gamev1.BrowseRequest{NpcName: inst.Name()})
    require.NoError(t, err)
    require.NotNil(t, evt)
    assert.Contains(t, evt.GetMessage().Content, "stim_pack")
}

func TestHandleBrowse_UnknownPlayer(t *testing.T) {
    svc, _, inst := newMerchantTestServer(t)
    evt, err := svc.handleBrowse("nobody", &gamev1.BrowseRequest{NpcName: inst.Name()})
    require.NoError(t, err)
    assert.Contains(t, evt.GetMessage().Content, "not found")
}

func TestHandleBrowse_NpcNotFound(t *testing.T) {
    svc, uid, _ := newMerchantTestServer(t)
    evt, err := svc.handleBrowse(uid, &gamev1.BrowseRequest{NpcName: "ghost"})
    require.NoError(t, err)
    assert.Contains(t, evt.GetMessage().Content, "don't see")
}

func TestHandleBuy_SuccessDeductsCreditsAndStock(t *testing.T) {
    svc, uid, inst := newMerchantTestServer(t)
    evt, err := svc.handleBuy(uid, &gamev1.BuyRequest{NpcName: inst.Name(), ItemId: "stim_pack", Quantity: 1})
    require.NoError(t, err)
    assert.Contains(t, evt.GetMessage().Content, "buy")

    sess, _ := svc.sessions.GetPlayer(uid)
    assert.Equal(t, 450, sess.Currency)

    state := svc.merchantStateFor(inst.ID)
    require.NotNil(t, state)
    assert.Equal(t, 2, state.Stock["stim_pack"])
}

func TestHandleBuy_InsufficientCredits(t *testing.T) {
    svc, uid, inst := newMerchantTestServer(t)
    sess, _ := svc.sessions.GetPlayer(uid)
    sess.Currency = 10
    evt, err := svc.handleBuy(uid, &gamev1.BuyRequest{NpcName: inst.Name(), ItemId: "stim_pack", Quantity: 1})
    require.NoError(t, err)
    assert.Contains(t, evt.GetMessage().Content, "afford")
}

func TestHandleBuy_OutOfStock(t *testing.T) {
    svc, uid, inst := newMerchantTestServer(t)
    svc.merchantStateFor(inst.ID).Stock["stim_pack"] = 0
    evt, err := svc.handleBuy(uid, &gamev1.BuyRequest{NpcName: inst.Name(), ItemId: "stim_pack", Quantity: 1})
    require.NoError(t, err)
    assert.Contains(t, evt.GetMessage().Content, "out of stock")
}

func TestHandleSell_SuccessPaysPlayerAndDecrementsBudget(t *testing.T) {
    svc, uid, inst := newMerchantTestServer(t)
    evt, err := svc.handleSell(uid, &gamev1.SellRequest{NpcName: inst.Name(), ItemId: "stim_pack", Quantity: 1})
    require.NoError(t, err)
    assert.Contains(t, evt.GetMessage().Content, "buy")

    sess, _ := svc.sessions.GetPlayer(uid)
    assert.Equal(t, 525, sess.Currency) // 500 + floor(50 * 0.5 * 1) = 525

    state := svc.merchantStateFor(inst.ID)
    assert.Equal(t, 275, state.CurrentBudget)
}

func TestHandleSell_BudgetExhausted(t *testing.T) {
    svc, uid, inst := newMerchantTestServer(t)
    svc.merchantStateFor(inst.ID).CurrentBudget = 0
    evt, err := svc.handleSell(uid, &gamev1.SellRequest{NpcName: inst.Name(), ItemId: "stim_pack", Quantity: 1})
    require.NoError(t, err)
    assert.Contains(t, evt.GetMessage().Content, "can't afford")
}

func TestHandleNegotiate_OnlyOncePerVisit(t *testing.T) {
    svc, uid, inst := newMerchantTestServer(t)
    _, err := svc.handleNegotiate(uid, &gamev1.NegotiateRequest{NpcName: inst.Name(), Skill: "smooth_talk"})
    require.NoError(t, err)

    // Force already-negotiated state.
    sess, _ := svc.sessions.GetPlayer(uid)
    sess.NegotiatedMerchantID = inst.ID

    evt, err := svc.handleNegotiate(uid, &gamev1.NegotiateRequest{NpcName: inst.Name(), Skill: "smooth_talk"})
    require.NoError(t, err)
    assert.Contains(t, evt.GetMessage().Content, "already tried")
}

func TestHandleNegotiate_CoweringMerchantBlocked(t *testing.T) {
    svc, uid, inst := newMerchantTestServer(t)
    inst.Cowering = true
    evt, err := svc.handleNegotiate(uid, &gamev1.NegotiateRequest{NpcName: inst.Name()})
    require.NoError(t, err)
    assert.Contains(t, evt.GetMessage().Content, "cower")
}

func TestProperty_HandleBrowse_NeverPanics(t *testing.T) {
    svc, uid, _ := newMerchantTestServer(t)
    rapid.Check(t, func(rt *rapid.T) {
        name := rapid.String().Draw(rt, "name")
        evt, err := svc.handleBrowse(uid, &gamev1.BrowseRequest{NpcName: name})
        assert.NoError(t, err)
        assert.NotNil(t, evt)
    })
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run "TestHandleBrowse|TestHandleBuy|TestHandleSell|TestHandleNegotiate|TestProperty_HandleBrowse" -v 2>&1 | head -15
```

- [ ] **Step 3: Create `internal/gameserver/grpc_service_merchant.go`**

```go
package gameserver

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/cory-johannsen/mud/internal/game/npc"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

var merchantRuntimeMu sync.RWMutex

// initMerchantRuntimeState initialises runtime state for a merchant instance if absent.
//
// Precondition: inst must be non-nil with NPCType "merchant" and a registered template.
// Postcondition: merchantRuntimeStates[inst.ID] is set.
func (s *GameServiceServer) initMerchantRuntimeState(inst *npc.Instance) {
	if inst.NPCType != "merchant" {
		return
	}
	tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
	if tmpl == nil || tmpl.Merchant == nil {
		return
	}
	merchantRuntimeMu.Lock()
	defer merchantRuntimeMu.Unlock()
	if _, ok := s.merchantRuntimeStates[inst.ID]; !ok {
		s.merchantRuntimeStates[inst.ID] = npc.InitRuntimeState(tmpl.Merchant, time.Now())
	}
}

// merchantStateFor returns the MerchantRuntimeState for instID, or nil.
func (s *GameServiceServer) merchantStateFor(instID string) *npc.MerchantRuntimeState {
	merchantRuntimeMu.RLock()
	defer merchantRuntimeMu.RUnlock()
	return s.merchantRuntimeStates[instID]
}

// findMerchantInRoom returns the first non-cowering merchant NPC matching npcName in roomID.
func (s *GameServiceServer) findMerchantInRoom(roomID, npcName string) (*npc.Instance, string) {
	inst := s.npcMgr.FindInRoom(roomID, npcName)
	if inst == nil {
		return nil, fmt.Sprintf("You don't see %q here.", npcName)
	}
	if inst.NPCType != "merchant" {
		return nil, fmt.Sprintf("%s is not a merchant.", inst.Name())
	}
	if inst.Cowering {
		return nil, fmt.Sprintf("%s is cowering in fear and won't respond right now.", inst.Name())
	}
	return inst, ""
}

// handleBrowse lists a merchant's inventory with current prices.
//
// Precondition: uid is valid; req.NpcName is non-empty.
// Postcondition: Returns a message event listing items, or an error message.
func (s *GameServiceServer) handleBrowse(uid string, req *gamev1.BrowseRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	inst, errMsg := s.findMerchantInRoom(sess.RoomID, req.GetNpcName())
	if inst == nil {
		return messageEvent(errMsg), nil
	}
	tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
	if tmpl == nil || tmpl.Merchant == nil {
		return messageEvent("This merchant has no inventory configured."), nil
	}
	state := s.merchantStateFor(inst.ID)
	if state == nil {
		s.initMerchantRuntimeState(inst)
		state = s.merchantStateFor(inst.ID)
	}
	surcharge := s.wantedSurchargeFor(sess, inst)
	rows := npc.BrowseLines(tmpl.Merchant, state, surcharge, sess.NegotiateModifier)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== %s's Wares ===\n", inst.Name()))
	sb.WriteString(fmt.Sprintf("%-20s %8s %8s %6s\n", "Item", "Buy", "Sell", "Stock"))
	for _, row := range rows {
		sb.WriteString(fmt.Sprintf("%-20s %8d %8d %6d\n", row.ItemID, row.BuyPrice, row.SellPrice, row.Stock))
	}
	return messageEvent(sb.String()), nil
}

// wantedSurchargeFor returns 1.1 if the player has WantedLevel 1 in the NPC's zone, else 1.0.
func (s *GameServiceServer) wantedSurchargeFor(sess *PlayerSession, inst *npc.Instance) float64 {
	room := s.world.RoomByID(sess.RoomID)
	if room == nil {
		return 1.0
	}
	if wl, ok := sess.WantedLevel[room.ZoneID]; ok && wl >= 1 {
		return 1.1
	}
	return 1.0
}

// handleBuy executes a player purchase from a merchant.
//
// Precondition: uid valid; item exists in merchant inventory; player has sufficient credits.
// Postcondition: sess.Currency decremented; stock decremented.
func (s *GameServiceServer) handleBuy(uid string, req *gamev1.BuyRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	inst, errMsg := s.findMerchantInRoom(sess.RoomID, req.GetNpcName())
	if inst == nil {
		return messageEvent(errMsg), nil
	}
	tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
	if tmpl == nil || tmpl.Merchant == nil {
		return messageEvent("This merchant has no inventory."), nil
	}
	state := s.merchantStateFor(inst.ID)
	if state == nil {
		s.initMerchantRuntimeState(inst)
		state = s.merchantStateFor(inst.ID)
	}
	itemID := req.GetItemId()
	qty := int(req.GetQuantity())
	if qty < 1 {
		qty = 1
	}
	var itemCfg *npc.MerchantItem
	for i := range tmpl.Merchant.Inventory {
		if tmpl.Merchant.Inventory[i].ItemID == itemID {
			itemCfg = &tmpl.Merchant.Inventory[i]
			break
		}
	}
	if itemCfg == nil {
		return messageEvent(fmt.Sprintf("%s doesn't sell %q.", inst.Name(), itemID)), nil
	}
	if state.Stock[itemID] < qty {
		return messageEvent(fmt.Sprintf("%s is out of stock on %s.", inst.Name(), itemID)), nil
	}
	surcharge := s.wantedSurchargeFor(sess, inst)
	unitPrice := npc.ComputeBuyPrice(itemCfg.BasePrice, tmpl.Merchant.SellMargin, surcharge, sess.NegotiateModifier)
	total := unitPrice * qty
	if sess.Currency < total {
		return messageEvent(fmt.Sprintf("You can't afford that. It costs %d credits and you have %d.", total, sess.Currency)), nil
	}
	merchantRuntimeMu.Lock()
	state.Stock[itemID] -= qty
	merchantRuntimeMu.Unlock()
	sess.Currency -= total
	return messageEvent(fmt.Sprintf("You buy %d× %s for %d credits.", qty, itemID, total)), nil
}

// handleSell executes a player sale to a merchant.
//
// Precondition: uid valid; merchant has sufficient budget.
// Postcondition: sess.Currency incremented; merchant budget decremented.
func (s *GameServiceServer) handleSell(uid string, req *gamev1.SellRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	inst, errMsg := s.findMerchantInRoom(sess.RoomID, req.GetNpcName())
	if inst == nil {
		return messageEvent(errMsg), nil
	}
	tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
	if tmpl == nil || tmpl.Merchant == nil {
		return messageEvent("This merchant doesn't buy anything."), nil
	}
	state := s.merchantStateFor(inst.ID)
	if state == nil {
		s.initMerchantRuntimeState(inst)
		state = s.merchantStateFor(inst.ID)
	}
	itemID := req.GetItemId()
	qty := int(req.GetQuantity())
	if qty < 1 {
		qty = 1
	}
	var itemCfg *npc.MerchantItem
	for i := range tmpl.Merchant.Inventory {
		if tmpl.Merchant.Inventory[i].ItemID == itemID {
			itemCfg = &tmpl.Merchant.Inventory[i]
			break
		}
	}
	if itemCfg == nil {
		return messageEvent(fmt.Sprintf("%s doesn't buy %q.", inst.Name(), itemID)), nil
	}
	payout := npc.ComputeSellPayout(itemCfg.BasePrice, tmpl.Merchant.BuyMargin, qty, sess.NegotiateModifier)
	merchantRuntimeMu.RLock()
	budget := state.CurrentBudget
	merchantRuntimeMu.RUnlock()
	if budget < payout {
		return messageEvent(fmt.Sprintf("%s can't afford to buy that right now.", inst.Name())), nil
	}
	merchantRuntimeMu.Lock()
	state.CurrentBudget -= payout
	merchantRuntimeMu.Unlock()
	sess.Currency += payout
	return messageEvent(fmt.Sprintf("%s buys %d× %s from you for %d credits.", inst.Name(), qty, itemID, payout)), nil
}

// handleNegotiate attempts a skill check for a session-scoped price modifier.
// REQ-NPC-5: only once per merchant per room visit.
//
// Precondition: uid valid; merchant non-cowering in same room.
// Postcondition: sess.NegotiatedMerchantID == inst.ID; sess.NegotiateModifier set.
func (s *GameServiceServer) handleNegotiate(uid string, req *gamev1.NegotiateRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	inst, errMsg := s.findMerchantInRoom(sess.RoomID, req.GetNpcName())
	if inst == nil {
		return messageEvent(errMsg), nil
	}
	if sess.NegotiatedMerchantID == inst.ID {
		return messageEvent(fmt.Sprintf("You've already tried negotiating with %s this visit.", inst.Name())), nil
	}
	dc := 10 + inst.Awareness
	roll := rand.Intn(20) + 1
	skillID := req.GetSkill()
	if skillID == "" {
		skillID = "smooth_talk"
	}
	skillMod := merchantSkillModifier(sess.Skills[skillID])
	total := roll + skillMod
	var outcome string
	switch {
	case total >= dc+10:
		outcome = "crit_success"
	case total >= dc:
		outcome = "success"
	case total <= dc-10:
		outcome = "crit_failure"
	default:
		outcome = "failure"
	}
	mod := npc.ApplyNegotiateOutcome(outcome)
	sess.NegotiateModifier = mod
	sess.NegotiatedMerchantID = inst.ID
	var msg string
	switch outcome {
	case "crit_success":
		msg = fmt.Sprintf("You charm %s brilliantly. Prices improved by 20%%!", inst.Name())
	case "success":
		msg = fmt.Sprintf("You negotiate with %s. Prices improved by 10%%.", inst.Name())
	case "crit_failure":
		msg = fmt.Sprintf("%s is insulted by your pitch. Buy prices increased by 10%%.", inst.Name())
	default:
		msg = fmt.Sprintf("%s shrugs off your pitch. No change.", inst.Name())
	}
	return messageEvent(msg), nil
}

// merchantSkillModifier converts a proficiency rank to a flat modifier.
// Ranks: "" or "untrained"=0, "trained"=2, "expert"=4, "master"=6, "legendary"=8.
func merchantSkillModifier(rank string) int {
	switch rank {
	case "trained":
		return 2
	case "expert":
		return 4
	case "master":
		return 6
	case "legendary":
		return 8
	default:
		return 0
	}
}
```

**Note:** `PlayerSession` may be an unexported type or accessed via pointer. Check the type returned by `s.sessions.GetPlayer` and adjust the `wantedSurchargeFor` signature accordingly. Also confirm the `WantedLevel` field name on the session struct.

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run "TestHandleBrowse|TestHandleBuy|TestHandleSell|TestHandleNegotiate|TestProperty_HandleBrowse" -v 2>&1 | tail -20
```

- [ ] **Step 5: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | tail -10
```

- [ ] **Step 6: Commit**

```bash
git add internal/gameserver/grpc_service_merchant.go internal/gameserver/grpc_service_merchant_test.go
git commit -m "feat: merchant handler methods — browse, buy, sell, negotiate (REQ-NPC-5, REQ-NPC-5a, REQ-NPC-5b, REQ-NPC-12)"
```

---

## Task 8: Banker handler methods

**Files:**
- Create: `internal/gameserver/grpc_service_banker.go`
- Create: `internal/gameserver/grpc_service_banker_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/gameserver/grpc_service_banker_test.go` (package `gameserver`):

```go
package gameserver

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "pgregory.net/rapid"

    "github.com/cory-johannsen/mud/internal/game/npc"
    "github.com/cory-johannsen/mud/internal/game/session"
    gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

func newBankerTestServer(t *testing.T) (*GameServiceServer, string, *npc.Instance) {
    t.Helper()
    svc := testServiceWithAdmin(t, nil)

    uid := "bank_u1"
    _, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
        UID: uid, Username: "bank_user", CharName: "BankChar",
        CharacterID: 1, RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
    })
    require.NoError(t, err)
    sess, _ := svc.sessions.GetPlayer(uid)
    sess.Currency = 200
    sess.StashBalance = 100

    tmpl := &npc.Template{
        ID: "test_banker", Name: "Vault Keeper", NPCType: "banker",
        MaxHP: 10, AC: 10, Level: 1,
        Banker: &npc.BankerConfig{ZoneID: "test", BaseRate: 1.0, RateVariance: 0.05},
    }
    inst, err := svc.npcMgr.Spawn(tmpl, "room_a")
    require.NoError(t, err)
    svc.bankerRuntimeStates[inst.ID] = &npc.BankerRuntimeState{CurrentRate: 1.0}

    return svc, uid, inst
}

func TestHandleStashDeposit_SuccessDeductsCreditsAddsStash(t *testing.T) {
    svc, uid, inst := newBankerTestServer(t)
    evt, err := svc.handleStashDeposit(uid, &gamev1.StashDepositRequest{NpcName: inst.Name(), Amount: 100})
    require.NoError(t, err)
    assert.Contains(t, evt.GetMessage().Content, "deposited")

    sess, _ := svc.sessions.GetPlayer(uid)
    assert.Equal(t, 100, sess.Currency)
    assert.Equal(t, 200, sess.StashBalance)
}

func TestHandleStashDeposit_InsufficientCredits(t *testing.T) {
    svc, uid, inst := newBankerTestServer(t)
    evt, err := svc.handleStashDeposit(uid, &gamev1.StashDepositRequest{NpcName: inst.Name(), Amount: 500})
    require.NoError(t, err)
    assert.Contains(t, evt.GetMessage().Content, "don't have")
}

func TestHandleStashWithdraw_SuccessDeductsStashAddsCredits(t *testing.T) {
    svc, uid, inst := newBankerTestServer(t)
    evt, err := svc.handleStashWithdraw(uid, &gamev1.StashWithdrawRequest{NpcName: inst.Name(), Amount: 50})
    require.NoError(t, err)
    assert.Contains(t, evt.GetMessage().Content, "withdrew")

    sess, _ := svc.sessions.GetPlayer(uid)
    assert.Equal(t, 50, sess.StashBalance)
    assert.Equal(t, 250, sess.Currency)
}

func TestHandleStashWithdraw_InsufficientStash(t *testing.T) {
    svc, uid, inst := newBankerTestServer(t)
    evt, err := svc.handleStashWithdraw(uid, &gamev1.StashWithdrawRequest{NpcName: inst.Name(), Amount: 500})
    require.NoError(t, err)
    assert.Contains(t, evt.GetMessage().Content, "don't have enough")
}

func TestHandleStashBalance_DisplaysRateAndBalance(t *testing.T) {
    svc, uid, inst := newBankerTestServer(t)
    evt, err := svc.handleStashBalance(uid, &gamev1.StashBalanceRequest{NpcName: inst.Name()})
    require.NoError(t, err)
    msg := evt.GetMessage().Content
    assert.Contains(t, msg, "100")
    assert.Contains(t, msg, "1.00")
}

func TestHandleStashDeposit_CoweringBankerBlocked(t *testing.T) {
    svc, uid, inst := newBankerTestServer(t)
    inst.Cowering = true
    evt, err := svc.handleStashDeposit(uid, &gamev1.StashDepositRequest{NpcName: inst.Name(), Amount: 10})
    require.NoError(t, err)
    assert.Contains(t, evt.GetMessage().Content, "cower")
}

func TestProperty_HandleStashDeposit_NeverPanics(t *testing.T) {
    svc, uid, inst := newBankerTestServer(t)
    rapid.Check(t, func(rt *rapid.T) {
        amt := rapid.Int32().Draw(rt, "amount")
        evt, err := svc.handleStashDeposit(uid, &gamev1.StashDepositRequest{NpcName: inst.Name(), Amount: amt})
        assert.NoError(t, err)
        assert.NotNil(t, evt)
    })
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run "TestHandleStash|TestProperty_HandleStash" -v 2>&1 | head -10
```

- [ ] **Step 3: Create `internal/gameserver/grpc_service_banker.go`**

```go
package gameserver

import (
	"fmt"
	"sync"

	"github.com/cory-johannsen/mud/internal/game/npc"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

var bankerRuntimeMu sync.RWMutex

// findBankerInRoom returns the first non-cowering banker NPC matching npcName in roomID.
func (s *GameServiceServer) findBankerInRoom(roomID, npcName string) (*npc.Instance, string) {
	inst := s.npcMgr.FindInRoom(roomID, npcName)
	if inst == nil {
		return nil, fmt.Sprintf("You don't see %q here.", npcName)
	}
	if inst.NPCType != "banker" {
		return nil, fmt.Sprintf("%s is not a banker.", inst.Name())
	}
	if inst.Cowering {
		return nil, fmt.Sprintf("%s is cowering in fear and won't respond right now.", inst.Name())
	}
	return inst, ""
}

// bankerRateFor returns the current exchange rate for a banker instance (default 1.0).
func (s *GameServiceServer) bankerRateFor(instID string) float64 {
	bankerRuntimeMu.RLock()
	defer bankerRuntimeMu.RUnlock()
	if st, ok := s.bankerRuntimeStates[instID]; ok {
		return st.CurrentRate
	}
	return 1.0
}

// handleStashDeposit deducts amount from carried credits and adds floor(amount*rate) to stash.
//
// REQ-NPC-14: uses CurrentRate at command execution time.
// Precondition: uid valid; amount > 0; player has sufficient currency.
// Postcondition: sess.Currency decremented; sess.StashBalance incremented.
func (s *GameServiceServer) handleStashDeposit(uid string, req *gamev1.StashDepositRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	inst, errMsg := s.findBankerInRoom(sess.RoomID, req.GetNpcName())
	if inst == nil {
		return messageEvent(errMsg), nil
	}
	amount := int(req.GetAmount())
	if amount <= 0 {
		return messageEvent("You must deposit a positive amount."), nil
	}
	if sess.Currency < amount {
		return messageEvent(fmt.Sprintf("You don't have %d credits to deposit.", amount)), nil
	}
	rate := s.bankerRateFor(inst.ID)
	stashAdded := npc.ComputeDeposit(amount, rate)
	sess.Currency -= amount
	sess.StashBalance += stashAdded
	return messageEvent(fmt.Sprintf(
		"You deposited %d credits. %d added to your stash (rate: %.2f). Stash balance: %d.",
		amount, stashAdded, rate, sess.StashBalance,
	)), nil
}

// handleStashWithdraw deducts amount from stash and adds floor(amount/rate) to carried credits.
//
// REQ-NPC-14: uses CurrentRate at command execution time.
// Precondition: uid valid; amount > 0; player has sufficient stash.
// Postcondition: sess.StashBalance decremented; sess.Currency incremented.
func (s *GameServiceServer) handleStashWithdraw(uid string, req *gamev1.StashWithdrawRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	inst, errMsg := s.findBankerInRoom(sess.RoomID, req.GetNpcName())
	if inst == nil {
		return messageEvent(errMsg), nil
	}
	amount := int(req.GetAmount())
	if amount <= 0 {
		return messageEvent("You must withdraw a positive amount."), nil
	}
	if sess.StashBalance < amount {
		return messageEvent(fmt.Sprintf("You don't have enough in your stash. Balance: %d.", sess.StashBalance)), nil
	}
	rate := s.bankerRateFor(inst.ID)
	creditsReceived := npc.ComputeWithdrawal(amount, rate)
	sess.StashBalance -= amount
	sess.Currency += creditsReceived
	return messageEvent(fmt.Sprintf(
		"You withdrew %d from your stash and received %d credits (rate: %.2f). Stash balance: %d.",
		amount, creditsReceived, rate, sess.StashBalance,
	)), nil
}

// handleStashBalance displays stash balance and the banker's current exchange rate.
//
// Precondition: uid valid; inst is a non-cowering banker in the same room.
// Postcondition: Returns message with balance and rate.
func (s *GameServiceServer) handleStashBalance(uid string, req *gamev1.StashBalanceRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	inst, errMsg := s.findBankerInRoom(sess.RoomID, req.GetNpcName())
	if inst == nil {
		return messageEvent(errMsg), nil
	}
	rate := s.bankerRateFor(inst.ID)
	return messageEvent(fmt.Sprintf(
		"Stash balance: %d credits.\n%s's current rate: %.2f.",
		sess.StashBalance, inst.Name(), rate,
	)), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run "TestHandleStash|TestProperty_HandleStash" -v 2>&1 | tail -15
```

- [ ] **Step 5: Run full suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | tail -10
```

- [ ] **Step 6: Commit**

```bash
git add internal/gameserver/grpc_service_banker.go internal/gameserver/grpc_service_banker_test.go
git commit -m "feat: banker handler methods — stash deposit, withdraw, balance (REQ-NPC-14)"
```

---

## Task 9: Wire dispatch cases, negotiate clearing, and calendar ticks

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/gameserver/world_handler.go` (or wherever room transition fires)

**Before starting:** Search for the dispatch switch in `grpc_service.go`:
```bash
grep -n "ClientMessage_Ready" /home/cjohannsen/src/mud/internal/gameserver/grpc_service.go
```

Also find where `sess.RoomID` is updated on movement:
```bash
grep -rn "RoomID =" /home/cjohannsen/src/mud/internal/gameserver/world_handler.go | head -10
```

- [ ] **Step 1: Write failing test for negotiate clearing**

Add to `internal/gameserver/grpc_service_merchant_test.go`:

```go
func TestNegotiateModifier_ClearedOnRoomTransition(t *testing.T) {
    svc, uid, _ := newMerchantTestServer(t)

    sess, _ := svc.sessions.GetPlayer(uid)
    sess.NegotiateModifier = 0.2
    sess.NegotiatedMerchantID = "some-merchant-id"

    // Simulate a room transition via the world handler move.
    // NOTE: requires world_handler to call clearNegotiateState on transition.
    // This test will fail until Step 3 below wires the clearing.
    svc.clearNegotiateState(sess)

    assert.Equal(t, 0.0, sess.NegotiateModifier)
    assert.Equal(t, "", sess.NegotiatedMerchantID)
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/gameserver/... -run TestNegotiateModifier_ClearedOnRoomTransition -v 2>&1 | head -10
```

Expected: FAIL — `clearNegotiateState` undefined.

- [ ] **Step 3: Add clearNegotiateState to `grpc_service_merchant.go`**

```go
// clearNegotiateState resets session-scoped negotiate fields on room transition.
// REQ-NPC-5a: modifier is cleared whenever the player moves to a new room.
//
// Precondition: sess must be non-nil.
// Postcondition: sess.NegotiateModifier == 0.0; sess.NegotiatedMerchantID == "".
func (s *GameServiceServer) clearNegotiateState(sess *PlayerSession) {
    sess.NegotiateModifier = 0.0
    sess.NegotiatedMerchantID = ""
}
```

Then in `world_handler.go`, find where the player's RoomID is updated (after the move is confirmed valid). After `sess.RoomID = newRoomID`, add:

```go
s.gameServer.clearNegotiateState(sess)
```

Or if the world handler doesn't have a `gameServer` reference, add the clearing directly in the session update:

```go
sess.NegotiateModifier = 0.0
sess.NegotiatedMerchantID = ""
```

**Note:** Check whether `WorldHandler` has a reference to `GameServiceServer` or whether negotiate clearing belongs in the session manager directly. Read `world_handler.go` before deciding which approach to use.

- [ ] **Step 4: Add dispatch cases in `grpc_service.go`**

In the `switch p := msg.Payload.(type)` block, after `case *gamev1.ClientMessage_Ready:`, add:

```go
case *gamev1.ClientMessage_Browse:
    return s.handleBrowse(uid, p.Browse)
case *gamev1.ClientMessage_Buy:
    return s.handleBuy(uid, p.Buy)
case *gamev1.ClientMessage_Sell:
    return s.handleSell(uid, p.Sell)
case *gamev1.ClientMessage_Negotiate:
    return s.handleNegotiate(uid, p.Negotiate)
case *gamev1.ClientMessage_StashDeposit:
    return s.handleStashDeposit(uid, p.StashDeposit)
case *gamev1.ClientMessage_StashWithdraw:
    return s.handleStashWithdraw(uid, p.StashWithdraw)
case *gamev1.ClientMessage_StashBalance:
    return s.handleStashBalance(uid, p.StashBalance)
```

- [ ] **Step 5: Add calendar tick methods**

Add to `internal/gameserver/grpc_service.go` (or new file `grpc_service_npc_ticks.go`):

```go
// tickMerchantReplenish advances all overdue merchant runtime states.
//
// Precondition: now is current wall time.
// Postcondition: All states with NextReplenishAt <= now are updated.
func (s *GameServiceServer) tickMerchantReplenish(now time.Time) {
    merchantRuntimeMu.Lock()
    defer merchantRuntimeMu.Unlock()
    for instID, state := range s.merchantRuntimeStates {
        if now.Before(state.NextReplenishAt) {
            continue
        }
        inst := s.npcMgr.InstanceByID(instID)
        if inst == nil {
            continue
        }
        tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
        if tmpl == nil || tmpl.Merchant == nil {
            continue
        }
        s.merchantRuntimeStates[instID] = npc.ApplyReplenish(tmpl.Merchant, state, 0)
    }
}

// tickBankerRates recalculates CurrentRate for all banker instances.
// Called once per in-game day (Hour == 0 tick).
//
// Postcondition: All bankerRuntimeStates have updated CurrentRate in [0.5, 1.0].
func (s *GameServiceServer) tickBankerRates() {
    bankerRuntimeMu.Lock()
    defer bankerRuntimeMu.Unlock()
    for instID, state := range s.bankerRuntimeStates {
        inst := s.npcMgr.InstanceByID(instID)
        if inst == nil {
            continue
        }
        tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
        if tmpl == nil || tmpl.Banker == nil {
            continue
        }
        variance := tmpl.Banker.RateVariance
        delta := (rand.Float64()*2 - 1) * variance
        state.CurrentRate = npc.NewCurrentRateFromDelta(tmpl.Banker, delta)
    }
}
```

Wire in `NewGameServiceServer`:

```go
if s.calendar != nil {
    s.calendar.Subscribe(func(dt GameDateTime) {
        s.tickMerchantReplenish(time.Now())
        if dt.Hour == 0 {
            s.tickBankerRates()
        }
    })
}
```

Add required imports: `"math/rand"`, `"time"`.

**Note:** Confirm the `GameCalendar` subscribe API and `GameDateTime` type before writing — check `internal/gameserver/calendar.go` or similar for exact method/type names.

- [ ] **Step 6: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | grep -E "FAIL|ok" | tail -20
```

Expected: all `ok`.

- [ ] **Step 7: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/gameserver/world_handler.go \
        internal/gameserver/grpc_service_merchant.go internal/gameserver/grpc_service_merchant_test.go
git commit -m "feat: wire merchant/banker dispatch, negotiate clearing on move, daily tick hooks (REQ-NPC-5, REQ-NPC-5a, REQ-NPC-12)"
```

---

## Task 10: Named NPC YAML files

**Files:** (6 new files under `content/npcs/`)

**Before starting:** Read one existing combat NPC YAML to confirm the exact fields. Then check if `content/npcs/` uses subdirectories or flat layout.

```bash
ls /home/cjohannsen/src/mud/content/npcs/ | head -5
cat /home/cjohannsen/src/mud/content/npcs/$(ls /home/cjohannsen/src/mud/content/npcs/ | head -1)
```

- [ ] **Step 1: Create `content/npcs/sergeant_mack.yaml`**

```yaml
id: sergeant_mack
name: Sergeant Mack
npc_type: merchant
description: >
  A grizzled veteran with a jaw like a steel trap. He runs the armory out of
  Last Stand Lodge with military efficiency.
max_hp: 40
ac: 14
level: 5
awareness: 4
personality: cowardly
merchant:
  merchant_type: weapons
  sell_margin: 1.25
  buy_margin: 0.45
  budget: 2000
  inventory:
    - item_id: combat_knife
      base_price: 80
      init_stock: 5
      max_stock: 8
    - item_id: sawed_off_shotgun
      base_price: 350
      init_stock: 2
      max_stock: 4
    - item_id: pipe_pistol
      base_price: 150
      init_stock: 3
      max_stock: 5
  replenish_rate:
    min_hours: 6
    max_hours: 12
    stock_refill: 1
    budget_refill: 500
```

- [ ] **Step 2: Create `content/npcs/slick_sally.yaml`**

```yaml
id: slick_sally
name: Slick Sally
npc_type: merchant
description: >
  A sharp-tongued trader. Her stall at the Rusty Oasis is stocked with every
  consumable a scavenger could need.
max_hp: 20
ac: 11
level: 3
awareness: 3
personality: cowardly
merchant:
  merchant_type: consumables
  sell_margin: 1.2
  buy_margin: 0.4
  budget: 1000
  inventory:
    - item_id: stim_pack
      base_price: 50
      init_stock: 8
      max_stock: 12
    - item_id: rad_tabs
      base_price: 30
      init_stock: 10
      max_stock: 15
    - item_id: dirty_water
      base_price: 10
      init_stock: 20
      max_stock: 30
  replenish_rate:
    min_hours: 4
    max_hours: 8
    stock_refill: 3
    budget_refill: 200
```

- [ ] **Step 3: Create `content/npcs/whiskey_joe.yaml`**

```yaml
id: whiskey_joe
name: Whiskey Joe
npc_type: merchant
description: >
  A barrel-chested man who smells permanently of fermented cactus juice.
  The Bottle Shack is his kingdom.
max_hp: 25
ac: 10
level: 2
awareness: 2
personality: cowardly
merchant:
  merchant_type: consumables
  sell_margin: 1.15
  buy_margin: 0.35
  budget: 600
  inventory:
    - item_id: rotgut_booze
      base_price: 15
      init_stock: 20
      max_stock: 30
    - item_id: stim_pack
      base_price: 50
      init_stock: 4
      max_stock: 8
    - item_id: packaged_food
      base_price: 20
      init_stock: 10
      max_stock: 15
  replenish_rate:
    min_hours: 8
    max_hours: 16
    stock_refill: 2
    budget_refill: 100
```

- [ ] **Step 4: Create `content/npcs/old_rusty.yaml`**

```yaml
id: old_rusty
name: Old Rusty
npc_type: merchant
description: >
  Nobody knows Old Rusty's real name or how long he's been squatting in The Heap.
  His prices are low and his questions are fewer.
max_hp: 15
ac: 10
level: 1
awareness: 1
personality: cowardly
merchant:
  merchant_type: consumables
  sell_margin: 1.1
  buy_margin: 0.5
  budget: 400
  inventory:
    - item_id: stim_pack
      base_price: 50
      init_stock: 3
      max_stock: 6
    - item_id: dirty_water
      base_price: 10
      init_stock: 15
      max_stock: 20
    - item_id: scrap_bandage
      base_price: 8
      init_stock: 10
      max_stock: 15
  replenish_rate:
    min_hours: 12
    max_hours: 20
    stock_refill: 1
    budget_refill: 80
```

- [ ] **Step 5: Create `content/npcs/herb.yaml`**

```yaml
id: herb
name: Herb
npc_type: merchant
description: >
  A former combat medic who now peddles salvaged medical supplies out of a
  corrugated metal shack called The Green Hell.
max_hp: 30
ac: 12
level: 4
awareness: 4
personality: cowardly
merchant:
  merchant_type: consumables
  sell_margin: 1.3
  buy_margin: 0.45
  budget: 1500
  inventory:
    - item_id: stim_pack
      base_price: 50
      init_stock: 10
      max_stock: 20
    - item_id: trauma_kit
      base_price: 200
      init_stock: 3
      max_stock: 5
    - item_id: anti_rad_serum
      base_price: 120
      init_stock: 5
      max_stock: 8
  replenish_rate:
    min_hours: 6
    max_hours: 12
    stock_refill: 2
    budget_refill: 300
```

- [ ] **Step 6: Create `content/npcs/vera_coldcoin.yaml`**

```yaml
id: vera_coldcoin
name: Vera Coldcoin
npc_type: banker
description: >
  A thin woman in a crisp shirt who looks absurdly out of place in Rustbucket Ridge.
  She operates a reinforced strongbox out of a Safe room and speaks exclusively in numbers.
max_hp: 15
ac: 10
level: 3
awareness: 3
personality: cowardly
banker:
  zone_id: rustbucket_ridge
  base_rate: 0.92
  rate_variance: 0.05
```

- [ ] **Step 7: Verify YAML files load**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./internal/game/npc/... -run TestProperty_AllExistingNPCTemplatesStillLoad -v 2>&1
```

Expected: PASS — new YAML files load with correct NPCType.

- [ ] **Step 8: Commit**

```bash
git add content/npcs/sergeant_mack.yaml content/npcs/slick_sally.yaml \
        content/npcs/whiskey_joe.yaml content/npcs/old_rusty.yaml \
        content/npcs/herb.yaml content/npcs/vera_coldcoin.yaml
git commit -m "content: add named Rustbucket Ridge merchant and banker NPC YAML files"
```

---

## Task 11: Documentation update

**Files:**
- Modify: `docs/features/non-combat-npcs.md`

- [ ] **Step 1: Mark economic NPC requirements complete**

In `docs/features/non-combat-npcs.md`, mark these requirements as `[x]`:

- REQ-NPC-5: negotiate once per merchant room visit
- REQ-NPC-5a: negotiate modifier on session state, cleared on room exit
- REQ-NPC-5b: WantedLevel surcharge + negotiate modifier interaction
- REQ-NPC-12: merchant runtime state management and replenishment
- REQ-NPC-14: banker deposit/withdrawal at CurrentRate
- `browse`, `buy`, `sell`, `negotiate` commands
- Named merchants: Sergeant Mack, Slick Sally, Whiskey Joe, Old Rusty, Herb
- Global stash (`StashBalance`)
- `deposit`, `withdraw`, `balance` commands
- Named banker: Vera Coldcoin

- [ ] **Step 2: Run full test suite one final time**

```bash
cd /home/cjohannsen/src/mud && mise exec -- go test ./... 2>&1 | grep -E "FAIL|ok"
```

Expected: all `ok`.

- [ ] **Step 3: Commit**

```bash
git add docs/features/non-combat-npcs.md
git commit -m "docs: mark economic NPC requirements complete (sub-project 2)"
```

---

## Completion Checklist

- [ ] `NegotiateModifier`, `NegotiatedMerchantID`, `StashBalance` on PlayerSession
- [ ] Merchant pure logic: InitRuntimeState, ComputeBuyPrice, ComputeSellPayout, ApplyReplenish, BrowseLines, ApplyNegotiateOutcome
- [ ] Banker pure logic: BankerRuntimeState, NewCurrentRateFromDelta, ComputeDeposit, ComputeWithdrawal
- [ ] Proto: 7 new messages generated
- [ ] npc.Manager: TemplateByID, InstanceByID
- [ ] GameServiceServer: merchantRuntimeStates, bankerRuntimeStates fields
- [ ] Merchant handlers: handleBrowse, handleBuy, handleSell, handleNegotiate
- [ ] Banker handlers: handleStashDeposit, handleStashWithdraw, handleStashBalance
- [ ] Dispatch cases wired in grpc_service.go
- [ ] Negotiate fields cleared on room transition
- [ ] Daily tick hooks: tickMerchantReplenish, tickBankerRates
- [ ] 6 named NPC YAML files
- [ ] All tests pass: `go test ./...`

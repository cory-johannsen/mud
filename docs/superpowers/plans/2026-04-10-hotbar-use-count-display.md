# Plan: Hotbar Use Count Display and Recharge Hover

**GitHub Issue:** cory-johannsen/mud#20
**Spec:** `docs/superpowers/specs/2026-04-10-hotbar-use-count-display.md`
**Date:** 2026-04-10

---

## Step 1 — Proto: extend HotbarSlot (REQ-1)

**File:** `api/proto/game/v1/game.proto`

Add three fields to `HotbarSlot` after field 4:
```proto
int32  uses_remaining    = 5;
int32  max_uses          = 6;
string recharge_condition = 7;
```

Regenerate `api/proto/game/v1/game.pb.go`:
```bash
cd api/proto/game/v1 && protoc --go_out=. --go_opt=paths=source_relative game.proto
```

**TDD:** Add a test in `internal/gameserver/grpc_service_hotbar_test.go` asserting that a hotbar slot with a limited-use feat carries non-zero `UsesRemaining`/`MaxUses` fields.

---

## Step 2 — FeatDef: add RechargeCondition field (REQ-2c)

**Files:** `internal/gameserver/feat_registry.go` (or wherever `FeatDef` is defined), `content/feats.yaml`

- Add `RechargeCondition string` to the `FeatDef` Go struct
- Add `recharge_condition:` to the YAML loader
- Populate `recharge_condition:` in `content/feats.yaml` for every feat that has `active: true` and `prepared_uses > 0` (or any limited-use feat). Use values such as:
  - `"Recharges on rest"`
  - `"1 per combat"`
  - `"Daily"`

**TDD:** Add a test that loads `feats.yaml` and asserts every active limited-use feat has a non-empty `RechargeCondition`.

---

## Step 3 — Technology definitions: recharge_condition (REQ-2a)

**Files:** technology registry / YAML definitions

- Identify the Go struct for technology definitions and add `RechargeCondition string`
- Populate `recharge_condition:` in technology YAML definitions for innate, prepared, and spontaneous techs that have use limits:
  - Prepared: `"Recharges on rest"`
  - Innate with MaxUses: `"Recharges on rest"` (or per-tech as appropriate)
  - Spontaneous: `"Recharges on rest"` (pool-level)

**TDD:** Add a test asserting that all innate techs with `MaxUses > 0` have a non-empty `RechargeCondition`.

---

## Step 4 — Server: populate use state in resolveHotbarSlotDisplay (REQ-2a)

**File:** `internal/gameserver/grpc_service_hotbar.go`

Extend `resolveHotbarSlotDisplay` (line 115) to return `usesRemaining int32`, `maxUses int32`, `rechargeCondition string` in addition to `displayName` and `description`. Populate based on slot kind using the session state:

| Kind | uses_remaining | max_uses | recharge_condition |
|------|---------------|----------|--------------------|
| `feat` | `sess.ActiveFeatUses[ref]` | `featDef.PreparedUses` | `featDef.RechargeCondition` |
| `technology` (innate) | `sess.InnateTechs[ref].UsesRemaining` | `sess.InnateTechs[ref].MaxUses` | `techDef.RechargeCondition` |
| `technology` (prepared) | count non-expended slots for `ref` in `sess.PreparedTechs` | total prepared slots for `ref` | `techDef.RechargeCondition` |
| `technology` (spontaneous) | `sess.SpontaneousUsePools[level].Remaining` | `sess.SpontaneousUsePools[level].Max` | `techDef.RechargeCondition` |
| `command`/`consumable` | 0 | 0 | `""` |

Update `hotbarUpdateEvent` (line 94) to set these new fields on the `gamev1.HotbarSlot` proto message.

**TDD:** Unit tests for each slot kind asserting correct use state population. Property-based test asserting `MaxUses == 0` for all command/consumable slots.

---

## Step 5 — Server: re-send HotbarUpdateEvent after activation (REQ-2b)

**File:** `internal/gameserver/grpc_service.go`

After any activation path that decrements use counts in `handleUse` (lines ~7818 for feat uses, ~8029 for prepared tech expended, ~8080 for spontaneous pool), push an updated `HotbarUpdateEvent` to the session's event stream so the client receives current use counts immediately.

Pattern (after decrementing):
```go
if hotbarEvent := s.hotbarUpdateEvent(sess.Hotbar); hotbarEvent != nil {
    s.pushEvent(uid, hotbarEvent)
}
```

Also push after rest (motel/brothel rest handlers) when use pools are restored.

**TDD:** Test that after feat activation, the returned event stream includes a `HotbarUpdateEvent` with decremented `UsesRemaining`. Test that after rest, `HotbarUpdateEvent` is pushed with restored counts.

---

## Step 6 — TypeScript: update HotbarSlot type (REQ-3, REQ-4 prerequisite)

**File:** `cmd/webclient/ui/src/proto/index.ts`

Add new fields to the `HotbarSlot` interface:
```typescript
export interface HotbarSlot {
  kind: string
  ref: string
  displayName?: string
  display_name?: string
  description?: string
  usesRemaining?: number
  uses_remaining?: number
  maxUses?: number
  max_uses?: number
  rechargeCondition?: string
  recharge_condition?: string
}
```

---

## Step 7 — Web UI: use count badge on hotbar buttons (REQ-3)

**File:** `cmd/webclient/ui/src/game/panels/HotbarPanel.tsx`

In the slot button JSX (around line 217), add a badge element when `max_uses > 0`:

```typescript
const maxUses = slot.maxUses ?? slot.max_uses ?? 0
const usesRemaining = slot.usesRemaining ?? slot.uses_remaining ?? 0
const isExpended = maxUses > 0 && usesRemaining === 0
```

- Render `<span className="hotbar-use-badge">{usesRemaining}/{maxUses}</span>` inside the button when `maxUses > 0`
- Add CSS to `src/styles/game.css`: badge positioned `bottom: 2px; right: 2px`, small font, contrasting color
- Apply `hotbar-slot-expended` CSS class when `isExpended`, setting `opacity: 0.45`

---

## Step 8 — Web UI: replace title tooltip with portal tooltip (REQ-4)

**File:** `cmd/webclient/ui/src/game/panels/HotbarPanel.tsx`

Create a `HotbarTooltip` component modelled on `RoomTooltip.tsx`:
- Portal-rendered into `document.body`
- Fixed position, viewport-clamped
- Styled: `#1a1a1a` background, `#444` border, monospace font, padding 8px
- Content sections:
  1. **Name** — `slot.displayName ?? slot.display_name ?? slot.ref` (white)
  2. **Description** — if present (color `#ccc`)
  3. **Uses** — `{usesRemaining} / {maxUses} uses remaining` (color `#e0c060`) — only when `maxUses > 0`
  4. **Recharge** — `rechargeCondition` (color `#888`) — only when non-empty
  5. **Edit hint** — `right-click to edit` (color `#666`)

Replace `title={slotTooltip(slot)}` on line 227 with `onMouseEnter`/`onMouseLeave` handlers that show/hide `HotbarTooltip` at the cursor position.

Remove the now-unused `slotTooltip` function.

---

## Step 9 — Run full test suite and verify

```bash
mise exec -- go test ./internal/gameserver/... -count=1
cd cmd/webclient/ui && npm run build
```

All tests must pass. Build must succeed with no TypeScript errors.

---

## Dependency Order

```
Step 1 (proto) ──┬──▶ Step 4 (server populate)
Step 2 (featDef) ┘
Step 3 (techDef) ──▶ Step 4

Step 4 ──▶ Step 5 (re-send after activation)

Step 1 ──▶ Step 6 (TS types) ──┬──▶ Step 7 (badge)
                                └──▶ Step 8 (tooltip)

Step 5 + Step 7 + Step 8 ──▶ Step 9 (test suite)
```

Steps 2 and 3 are independent and can run in parallel with each other and with Step 1.
Steps 7 and 8 are independent and can run in parallel after Step 6.

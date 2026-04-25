# Circumstance, Item, and Status Bonuses and Penalties — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the four remaining gaps in the typed-bonus pipeline shipped by #245: route armor AC through `EffectSet` as item-typed; build a web Effects panel + per-stat breakdown tooltips on AC/attack/saves; route `OverrideNarrativeEvents` into the combat log with per-tick dedup and burst collapse; lock the cross-spec contract that #247 (cover), #252 (off-guard), #254 (detection-state off-guard), #258 (drug buffs), and #261 (runes) consume.

**Spec:** [docs/superpowers/specs/2026-04-25-bonus-types.md](https://github.com/cory-johannsen/mud/blob/main/docs/superpowers/specs/2026-04-25-bonus-types.md) (PR [#286](https://github.com/cory-johannsen/mud/pull/286))

**Architecture:** Four small surgical changes on top of the already-shipped #245 pipeline. (1) `BuildCombatantEffects` adds an item-typed AC bonus from equipped armor and sets the legacy `Combatant.ACMod` to `Resolve(StatAC).Total - baseAC` so callers that haven't migrated still see the right number. (2) A new `effects_log.go` wraps `OverrideNarrativeEvents`, dedups per `(effect, stat, transition)` per tick, and collapses bursts of >3 overrides into a single summary line; the surface routes through the existing combat log so both telnet and web see them. (3) New `GetEffects` and `GetEffectsDetail` gRPC RPCs; new web `EffectsPanel` and `StatTooltip` components; a new telnet `effects` / `effects detail <stat>` command set. (4) The cross-spec contracts (BTYPE-16..20) are documented in `docs/architecture/effects.md` so the consuming tickets land cleanly without each re-spec'ing the source ID convention.

**Tech Stack:** Go (`internal/game/combat/`, `internal/game/effect/`, `internal/gameserver/`), `pgregory.net/rapid` (armor swap property test), protobuf, telnet, React/TypeScript (`cmd/webclient/ui/src/game/character/`).

**Prerequisite:** #245 is merged (PR #274 — confirmed at planning time). #265 (content migration) is the follow-on that prunes the legacy `Combatant.ACMod` field and runs concurrently with this plan.

**Note on spec PR**: Spec is on PR #286, not yet merged. Plan PR depends on spec PR landing first.

---

## File Map

| Action | Path |
|--------|------|
| Modify | `internal/game/combat/combatant_effects.go` (armor migration; `OverrideNarrativeEvents` routing) |
| Modify | `internal/game/combat/combatant_effects_test.go` |
| Create | `internal/game/combat/testdata/rapid/TestArmorACBonus_Property/` |
| Create | `internal/game/combat/effects_log.go` (transition-event emitter with dedup + burst collapse) |
| Create | `internal/game/combat/effects_log_test.go` |
| Modify | `internal/game/effect/render/render.go` (`EffectDetail(set, stat)` helper) |
| Modify | `internal/game/effect/render/render_test.go` |
| Modify | `internal/gameserver/grpc_service.go` (`GetEffects`, `GetEffectsDetail` RPCs) |
| Modify | `api/proto/game/v1/game.proto` (`EffectsView`, `EffectView`, `BonusView`, `EffectDetailView`) |
| Create | `cmd/webclient/ui/src/game/character/EffectsPanel.tsx` |
| Create | `cmd/webclient/ui/src/game/character/EffectsPanel.test.tsx` |
| Create | `cmd/webclient/ui/src/game/character/StatTooltip.tsx` |
| Create | `cmd/webclient/ui/src/game/character/StatTooltip.test.tsx` |
| Modify | `cmd/webclient/ui/src/game/character/CharacterSheet.tsx` (wire StatTooltip) |
| Modify | `internal/frontend/telnet/effects_handler.go` (or create) |
| Modify | `internal/frontend/telnet/effects_handler_test.go` (or create) |
| Modify | `docs/architecture/effects.md` (cross-spec contract section) |

---

### Task 1: Armor AC migration

**Files:**
- Modify: `internal/game/combat/combatant_effects.go`
- Modify: `internal/game/combat/combatant_effects_test.go`
- Create: `internal/game/combat/testdata/rapid/TestArmorACBonus_Property/`

- [ ] **Step 1: Failing tests** (BTYPE-1, BTYPE-2, BTYPE-3, BTYPE-4):

```go
func TestBuildCombatantEffects_ArmorEmitsItemTypedACBonus(t *testing.T) {
    c := newCombatant(t, withArmor("leather", acBonus: 2))
    combat.BuildCombatantEffects(c)
    res := effect.Resolve(c.Effects, effect.StatAC)
    var found *effect.Contribution
    for _, ct := range res.Contributions {
        if ct.Bonus.SourceID == "item:leather" {
            found = &ct
            break
        }
    }
    require.NotNil(t, found, "armor must contribute an item-typed AC bonus (BTYPE-1)")
    require.Equal(t, effect.BonusItem, found.Bonus.Type)
    require.Equal(t, 2, found.Bonus.Value)
    require.True(t, found.Active)
}

func TestBuildCombatantEffects_LegacyACModEqualsResolvedDelta(t *testing.T) {
    c := newCombatant(t, withArmor("leather", acBonus: 2), withConditionThatGivesCircumstanceAC(1))
    combat.BuildCombatantEffects(c)
    res := effect.Resolve(c.Effects, effect.StatAC)
    require.Equal(t, res.Total - c.AC, c.ACMod, "BTYPE-2 legacy field equals resolved delta")
}

func TestBuildCombatantEffects_NoDoubleCountingViaACModAndResolve(t *testing.T) {
    c := newCombatant(t, withArmor("leather", acBonus: 2))
    combat.BuildCombatantEffects(c)
    res := effect.Resolve(c.Effects, effect.StatAC)
    // BTYPE-3: the resolved total IS authoritative; ACMod is a derived legacy view.
    require.Equal(t, c.AC + 2, res.Total)
    // Legacy ACMod is reachable but additive use of it should never happen — a regression
    // would manifest as res.Total + c.ACMod = c.AC + 4.
    require.NotEqual(t, c.AC + 4, res.Total + c.ACMod-c.ACMod /*sanity*/, "value must not double-count")
}
```

- [ ] **Step 2: Implement** the armor emission in `BuildCombatantEffects`:

```go
if armor := c.EquippedArmor(); armor != nil && armor.ACBonus != 0 {
    c.Effects.Apply(effect.Bonus{
        Type:      effect.BonusItem,
        Stat:      effect.StatAC,
        Value:     armor.ACBonus,
        SourceID:  "item:" + armor.ID,
        CasterUID: c.UID,
    })
}
// After all bonuses are applied, set legacy ACMod for back-compat.
res := effect.Resolve(c.Effects, effect.StatAC)
c.ACMod = res.Total - c.AC
```

- [ ] **Step 3: Property test** (BTYPE-5):

```go
func TestProperty_ArmorACBonus_SwapsRecompute(t *testing.T) {
    rapid.Check(t, func(t *rapid.T) {
        baseAC := rapid.IntRange(8, 18).Draw(t, "baseAC")
        armorBonus := rapid.IntRange(0, 8).Draw(t, "armor")
        condBonus := rapid.IntRange(-2, 4).Draw(t, "cond")
        c := newCombatantWithArmor(baseAC, armorBonus)
        addCircumstanceCondition(c, condBonus)
        combat.BuildCombatantEffects(c)
        res := effect.Resolve(c.Effects, effect.StatAC)
        require.Equal(t, baseAC + armorBonus + condBonus, res.Total)
    })
}
```

- [ ] **Step 4: Audit `effect.Resolve(set, StatAC)` callers** in `round.go` and elsewhere; verify nobody adds `c.ACMod` on top after BTYPE-3. Update tests where they currently double-count.

---

### Task 2: Override-event routing — dedup + burst collapse

**Files:**
- Create: `internal/game/combat/effects_log.go`
- Create: `internal/game/combat/effects_log_test.go`
- Modify: `internal/game/combat/combatant_effects.go` (route `OverrideNarrativeEvents` through new emitter)

- [ ] **Step 1: Failing tests** (BTYPE-12, BTYPE-13, BTYPE-14, BTYPE-15, BTYPE-Q5):

```go
func TestEffectsLog_EmitsOnSuppressionTransition(t *testing.T) {
    log := captureLog(t)
    c := newCombatantWithStatusBonus("inspire_courage", value: 1)
    addStatusBonus(c, "heroism", value: 2) // overrides inspire_courage
    combat.RebuildEffects(c)
    require.Contains(t, log.LastLine(), "[EFFECT] inspire_courage")
    require.Contains(t, log.LastLine(), "is now suppressed by heroism")
}

func TestEffectsLog_NoEmitOnInitialApplication(t *testing.T) {
    log := captureLog(t)
    c := newCombatant(t)
    addStatusBonus(c, "inspire_courage", value: 1)
    combat.RebuildEffects(c)
    require.Empty(t, log.AllLines(), "BTYPE-15: initial application emits nothing")
}

func TestEffectsLog_DedupsWithinSingleTick(t *testing.T) {
    log := captureLog(t)
    c := newCombatant(t)
    addStatusBonus(c, "a", 1)
    addStatusBonus(c, "b", 2) // suppresses a
    addStatusBonus(c, "c", 3) // suppresses a and b
    combat.RebuildEffects(c) // single tick; both suppressions of `a` collapse
    counts := tallyByEffect(log.AllLines(), "a")
    require.Equal(t, 1, counts, "BTYPE-13 dedup within tick")
}

func TestEffectsLog_BurstCollapseAtThreshold(t *testing.T) {
    log := captureLog(t)
    c := newCombatant(t)
    addStatusBonuses(c, "small_a", "small_b", "small_c", "small_d") // all small
    addStatusBonus(c, "heroism", 5) // suppresses all four in one tick
    combat.RebuildEffects(c)
    require.Contains(t, log.LastLine(), "Heroism overrides 4 effects")
    require.NotContains(t, log.AllLines(), "small_a is now suppressed", "burst collapsed")
}

func TestEffectsLog_BothTelnetAndWebReceiveEvents(t *testing.T) {
    sink := combatLogSink()
    c := newCombatantWithStatusBonus("inspire", 1)
    addStatusBonus(c, "heroism", 3)
    combat.RebuildEffects(c)
    require.True(t, sink.HasTelnetLine("[EFFECT]"))
    require.True(t, sink.HasWebEvent("effect.transition"))
}
```

- [ ] **Step 2: Implement** the emitter:

```go
type EffectsLog struct {
    sink CombatLogSink
}

type Transition struct {
    EffectID    string
    Stat        effect.Stat
    From        TransitionState // active | suppressed
    To          TransitionState
    SuppressorID string
    CombatantUID string
}

func (l *EffectsLog) EmitTick(transitions []Transition) {
    seen := map[string]bool{}
    bySuppressor := map[string][]Transition{}
    for _, t := range transitions {
        key := fmt.Sprintf("%s:%s:%v→%v", t.EffectID, t.Stat, t.From, t.To)
        if seen[key] { continue }
        seen[key] = true
        if t.SuppressorID != "" {
            bySuppressor[t.SuppressorID] = append(bySuppressor[t.SuppressorID], t)
        }
    }
    for suppressor, ts := range bySuppressor {
        if len(ts) > 3 {
            l.sink.Emit(fmt.Sprintf("[EFFECT] %s overrides %d effects on %s", suppressor, len(ts), ts[0].CombatantUID))
            continue
        }
        for _, t := range ts {
            l.sink.Emit(fmt.Sprintf("[EFFECT] %s on %s: %s %+d %s is now suppressed by %s",
                t.EffectID, t.CombatantUID, t.Stat, valueOf(t), typeOf(t), suppressor))
        }
    }
}
```

- [ ] **Step 3: Wire** `OverrideNarrativeEvents` to call `EffectsLog.EmitTick(...)` once per `BuildCombatantEffects` invocation. The first build of an effect set after creation skips emission per BTYPE-15.

- [ ] **Step 4: Combat log routing** — both telnet (`combat console`) and web (combat-log slice) consume the same event stream via the existing combat-log sink interface.

---

### Task 3: `EffectDetail(set, stat)` helper + per-stat breakdown wire shape

**Files:**
- Modify: `internal/game/effect/render/render.go`
- Modify: `internal/game/effect/render/render_test.go`
- Modify: `api/proto/game/v1/game.proto`

- [ ] **Step 1: Add proto messages** (BTYPE-6, BTYPE-9):

```proto
message EffectsView {
  repeated EffectView effects = 1;
}

message EffectView {
  string id          = 1;
  string display_name = 2;
  string source      = 3;  // caster name when not self; empty otherwise
  repeated BonusView bonuses = 4;
}

message BonusView {
  string stat   = 1;
  int32  value  = 2;
  string type   = 3;  // "status" | "circumstance" | "item" | "untyped"
  bool   active = 4;
  string suppressed_by = 5;  // effect display name; empty when active
  string source_id  = 6;
}

message EffectDetailView {
  string stat = 1;
  int32  total = 2;
  repeated BonusView contributions = 3;  // active first, suppressed second
}

message GetEffectsRequest        { string character_id = 1; }
message GetEffectsResponse       { EffectsView view = 1; }

message GetEffectsDetailRequest  { string character_id = 1; string stat = 2; }
message GetEffectsDetailResponse { EffectDetailView view = 1; }
```

- [ ] **Step 2: Add `EffectDetail` helper** in `render/render.go`:

```go
func EffectDetail(set *effect.EffectSet, stat effect.Stat) EffectDetailView {
    res := effect.Resolve(set, stat)
    var contributions []BonusView
    for _, c := range res.Contributions {
        contributions = append(contributions, BonusView{
            Stat:         c.Bonus.Stat.String(),
            Value:        c.Bonus.Value,
            Type:         c.Bonus.Type.String(),
            Active:       c.Active,
            SuppressedBy: c.SuppressedByName, // resolved from suppressor source ID
            SourceId:     c.Bonus.SourceID,
        })
    }
    sort.SliceStable(contributions, func(i, j int) bool {
        if contributions[i].Active != contributions[j].Active {
            return contributions[i].Active // active first
        }
        return contributions[i].SourceId < contributions[j].SourceId
    })
    return EffectDetailView{Stat: stat.String(), Total: res.Total, Contributions: contributions}
}
```

- [ ] **Step 3: Failing tests**:

```go
func TestEffectDetail_OrdersActiveBeforeSuppressed(t *testing.T) {
    set := newSet(withStatusBonus("inspire", 1), withStatusBonus("heroism", 3))
    detail := render.EffectDetail(set, effect.StatAttack)
    require.Equal(t, "heroism", detail.Contributions[0].SourceId)
    require.True(t, detail.Contributions[0].Active)
    require.Equal(t, "inspire", detail.Contributions[1].SourceId)
    require.False(t, detail.Contributions[1].Active)
}

func TestEffectDetail_TotalMatchesResolve(t *testing.T) {
    set := newSet(withItemBonus("armor", 2), withCircumstanceBonus("cover", 1))
    detail := render.EffectDetail(set, effect.StatAC)
    require.Equal(t, 3, detail.Total)
}
```

---

### Task 4: gRPC handlers — `GetEffects` + `GetEffectsDetail`

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Create: `internal/gameserver/grpc_service_effects_test.go`

- [ ] **Step 1: Failing tests**:

```go
func TestGetEffects_ReturnsActiveEffectsForCharacter(t *testing.T) {
    s := newServer(t, withCharacterAndArmor("leather", 2))
    res, err := s.GetEffects(ctx, &gamev1.GetEffectsRequest{CharacterId: "c1"})
    require.NoError(t, err)
    require.Len(t, res.View.Effects, 1)
    require.Equal(t, "item:leather", res.View.Effects[0].Bonuses[0].SourceId)
}

func TestGetEffectsDetail_StatBreakdown(t *testing.T) {
    s := newServer(t, withCharacterAndArmor("leather", 2), withCondition("cover", BonusCircumstance, +1, StatAC))
    res, _ := s.GetEffectsDetail(ctx, &gamev1.GetEffectsDetailRequest{CharacterId: "c1", Stat: "ac"})
    require.Equal(t, baseAC+3, int(res.View.Total))
    require.Len(t, res.View.Contributions, 2)
}
```

- [ ] **Step 2: Implement** the handlers as thin wrappers over `render.EffectDetail` and the existing `EffectSet` snapshot.

---

### Task 5: Web `EffectsPanel` + `StatTooltip`

**Files:**
- Create: `cmd/webclient/ui/src/game/character/EffectsPanel.tsx`
- Create: `cmd/webclient/ui/src/game/character/EffectsPanel.test.tsx`
- Create: `cmd/webclient/ui/src/game/character/StatTooltip.tsx`
- Create: `cmd/webclient/ui/src/game/character/StatTooltip.test.tsx`
- Modify: `cmd/webclient/ui/src/game/character/CharacterSheet.tsx`

- [ ] **Step 1: Failing component tests** (BTYPE-6, BTYPE-7, BTYPE-8, BTYPE-9, BTYPE-10):

```ts
test("EffectsPanel renders effects grouped by source", () => {
  const view: EffectsView = { effects: [
    { id: "armor:leather", displayName: "Leather Armor", source: "", bonuses: [{ stat: "ac", value: 2, type: "item", active: true }] },
    { id: "cover:wooden_crate", displayName: "Cover", source: "", bonuses: [{ stat: "ac", value: 1, type: "circumstance", active: true }] },
  ]};
  render(<EffectsPanel view={view} />);
  expect(screen.getByText("Leather Armor")).toBeVisible();
  expect(screen.getByText(/AC \+2 \(item\)/)).toBeVisible();
});

test("EffectsPanel marks suppressed bonuses with suppressor in tooltip", () => {
  render(<EffectsPanel view={withSuppressedInspireUnderHeroism} />);
  const row = screen.getByText("Inspire Courage").closest("li")!;
  expect(within(row).getByText(/suppressed/i)).toBeVisible();
  fireEvent.mouseOver(within(row).getByLabelText("suppressed-by"));
  expect(screen.getByText(/overridden by Heroism/i)).toBeVisible();
});

test("StatTooltip shows total + contributions on hover", async () => {
  render(<StatTooltip stat="ac" detail={acDetail} />);
  fireEvent.mouseOver(screen.getByText(`AC: ${acDetail.total}`));
  expect(await screen.findByText("Leather Armor +2 item")).toBeVisible();
  expect(screen.getByText("Cover +1 circumstance")).toBeVisible();
});

test("StatTooltip refreshes on state change", async () => {
  const { rerender } = render(<StatTooltip stat="ac" detail={acDetailV1} />);
  rerender(<StatTooltip stat="ac" detail={acDetailV2} />);
  expect(screen.getByText(`AC: ${acDetailV2.total}`)).toBeVisible();
});
```

- [ ] **Step 2: Implement** the components. `StatTooltip` uses the existing tooltip primitive and dispatches `GetEffectsDetail` on first mount; subsequent state-change events from the combat slice trigger a re-fetch (BTYPE-Q2).

- [ ] **Step 3: Wire** `StatTooltip` on AC, attack, and saving-throw totals in `CharacterSheet.tsx`. Each total becomes a hover target.

---

### Task 6: Telnet `effects` and `effects detail <stat>` commands

**Files:**
- Create: `internal/frontend/telnet/effects_handler.go`
- Create: `internal/frontend/telnet/effects_handler_test.go`

- [ ] **Step 1: Failing tests** (BTYPE-11):

```go
func TestEffectsCommand_PrintsExistingBlock(t *testing.T) {
    h := newHandlerWithCharacter(t, withArmor("leather", 2), withCondition("inspire_courage"))
    out := h.Run("effects")
    require.Contains(t, out, "Leather Armor")
    require.Contains(t, out, "AC +2 item")
    require.Contains(t, out, "Inspire Courage")
}

func TestEffectsDetailCommand_AC(t *testing.T) {
    h := newHandlerWithCharacter(t, withArmor("leather", 2), withCondition("cover", BonusCircumstance, +1, StatAC))
    out := h.Run("effects detail ac")
    require.Contains(t, out, "AC total: ")
    require.Contains(t, out, "+2 item")
    require.Contains(t, out, "+1 circumstance")
}

func TestEffectsDetailCommand_RejectsUnknownStat(t *testing.T) {
    h := newHandlerWithCharacter(t)
    out := h.Run("effects detail bogus")
    require.Contains(t, out, "unknown stat")
}
```

- [ ] **Step 2: Implement** both commands using the existing `render.EffectsBlock(set)` and the new `render.EffectDetail(set, stat)`.

---

### Task 7: Cross-spec contract documentation

**Files:**
- Create: `docs/architecture/effects.md` (or extend if exists)

- [ ] **Step 1: Author** a section that names every consumer of the typed-bonus pipeline and the source-ID convention they MUST follow:

  - **Cover (#247)**: `Bonus{Type: circumstance, Stat: AC, SourceID: "cover:" + coverID, CasterUID: target.UID}`.
  - **Off-guard from skill actions (#252) / detection states (#254) / others**: `Bonus{Type: circumstance, Stat: AC, Value: -2, SourceID: "off_guard:<source>", CasterUID: ...}`.
  - **Frightened from skill actions (#252)**: `Bonus{Type: status, Stat: ..., Value: -stacks, SourceID: "frightened:<source>", CasterUID: ...}` per affected stat.
  - **Drug buffs (#258)**: each declared buff emits as a `Bonus` with author-chosen type; no special-case dispatch.
  - **Equipment runes / potency (#261)**: item-typed.

- [ ] **Step 2: Cross-link** to `internal/game/effect/`, the spec, and each consuming ticket. Document the lifecycle of `Combatant.ACMod` (deprecated, deletion targeted at end of #265).

- [ ] **Step 3:** Verify GitHub markdown preview renders cleanly.

---

## Verification

```
go test ./...
( cd cmd/webclient/ui && pnpm test )
```

Additional sanity:

- `go vet ./...` clean.
- `make proto` re-runs cleanly.
- Telnet smoke test: equip leather armor, verify `effects` shows it as item-typed; cast a status condition; trigger an override; verify the combat log emits `[EFFECT] ... is now suppressed by ...`; run `effects detail ac` and verify the breakdown.
- Web smoke test: open the character sheet; hover the AC total; verify the StatTooltip shows the breakdown; trigger a state change; verify the tooltip refreshes.

---

## Rollout / Open Questions Resolved at Plan Time

- **BTYPE-Q1**: `Combatant.ACMod` deletes immediately after #265 lands. Tracker comment in the field declaration.
- **BTYPE-Q2**: StatTooltip refreshes on state-change events; round tick is the fallback.
- **BTYPE-Q3**: Web Effects panel groups by source first, stat second. Mirrors telnet.
- **BTYPE-Q4**: Save-throw totals get the same breakdown tooltip treatment.
- **BTYPE-Q5**: Burst-collapse threshold is >3 transitions per suppressor per tick → single summary line.

## Non-Goals Reaffirmed

Per spec §2.2:

- No content migration of legacy fields (#265 owns it).
- No cover bonus model (#247 owns it).
- No new bonus types beyond the four PF2E types.
- No animations on bonus changes.
- No baseline rebalancing — migration preserves effective totals exactly.
- No admin UI for live editing.
